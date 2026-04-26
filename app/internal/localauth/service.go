package localauth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/frederic/tgtldr/app/internal/model"
	"github.com/frederic/tgtldr/app/internal/store"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrPasswordNotConfigured = errors.New("local auth password not configured")
	ErrInvalidPassword       = errors.New("invalid password")
	ErrSessionNotFound       = errors.New("session not found")
	ErrPasswordAlreadySet    = errors.New("password already configured")
	ErrPasswordTooShort      = errors.New("password too short")
	ErrPasswordTooLong       = errors.New("password too long")
)

const (
	MinPasswordLength = 8
	MaxPasswordLength = 128
	SessionTTL        = 30 * 24 * time.Hour

	maxFailedLoginAttempts = 5
	loginLockoutDuration   = time.Minute
)

type Service struct {
	store        *store.Store
	now          func() time.Time
	loginLimiter *loginLimiter
}

type LoginRateLimitError struct {
	retryAfter time.Duration
}

func (e *LoginRateLimitError) Error() string {
	return fmt.Sprintf("登录失败次数过多，请在 %d 秒后重试。", e.RetryAfterSeconds())
}

func (e *LoginRateLimitError) RetryAfterSeconds() int {
	if e.retryAfter <= 0 {
		return 1
	}
	return int((e.retryAfter + time.Second - 1) / time.Second)
}

type loginLimiter struct {
	mu          sync.Mutex
	failures    int
	lockedUntil time.Time
}

func NewService(st *store.Store) *Service {
	return &Service{
		store:        st,
		now:          time.Now,
		loginLimiter: &loginLimiter{},
	}
}

func (s *Service) PasswordConfigured(ctx context.Context) (bool, error) {
	auth, err := s.store.LocalAuth.Get(ctx)
	if err != nil {
		return false, err
	}
	return auth != nil && strings.TrimSpace(auth.PasswordHash) != "", nil
}

func (s *Service) SetupPassword(ctx context.Context, password string) (model.LocalSession, error) {
	current, err := s.store.LocalAuth.Get(ctx)
	if err != nil {
		return model.LocalSession{}, err
	}
	if current != nil && strings.TrimSpace(current.PasswordHash) != "" {
		return model.LocalSession{}, ErrPasswordAlreadySet
	}
	if err := validatePassword(password); err != nil {
		return model.LocalSession{}, err
	}

	now := s.now()
	hash, err := hashPassword(password)
	if err != nil {
		return model.LocalSession{}, err
	}
	if err := s.store.LocalAuth.Save(ctx, model.LocalAuth{
		PasswordHash:      hash,
		PasswordUpdatedAt: now,
		SessionVersion:    1,
	}); err != nil {
		return model.LocalSession{}, err
	}

	return s.createSession(ctx, 1, now)
}

func (s *Service) Login(ctx context.Context, password string) (model.LocalSession, error) {
	current, err := s.store.LocalAuth.Get(ctx)
	if err != nil {
		return model.LocalSession{}, err
	}
	if current == nil || strings.TrimSpace(current.PasswordHash) == "" {
		return model.LocalSession{}, ErrPasswordNotConfigured
	}
	now := s.now()
	if err := s.loginLimiter.beforeLogin(now); err != nil {
		return model.LocalSession{}, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(current.PasswordHash), []byte(password)); err != nil {
		s.loginLimiter.recordFailure(now)
		return model.LocalSession{}, ErrInvalidPassword
	}
	s.loginLimiter.recordSuccess()

	if err := s.store.LocalSessions.DeleteByVersionMismatch(ctx, current.SessionVersion); err != nil {
		return model.LocalSession{}, err
	}
	return s.createSession(ctx, current.SessionVersion, now)
}

func (s *Service) ChangePassword(ctx context.Context, currentPassword, newPassword string, currentSessionID string) (model.LocalSession, error) {
	current, err := s.store.LocalAuth.Get(ctx)
	if err != nil {
		return model.LocalSession{}, err
	}
	if current == nil || strings.TrimSpace(current.PasswordHash) == "" {
		return model.LocalSession{}, ErrPasswordNotConfigured
	}
	if err := bcrypt.CompareHashAndPassword([]byte(current.PasswordHash), []byte(currentPassword)); err != nil {
		return model.LocalSession{}, ErrInvalidPassword
	}
	if err := validatePassword(newPassword); err != nil {
		return model.LocalSession{}, err
	}

	now := s.now()
	hash, err := hashPassword(newPassword)
	if err != nil {
		return model.LocalSession{}, err
	}
	nextVersion := current.SessionVersion + 1
	if err := s.store.LocalAuth.Save(ctx, model.LocalAuth{
		PasswordHash:      hash,
		PasswordUpdatedAt: now,
		SessionVersion:    nextVersion,
	}); err != nil {
		return model.LocalSession{}, err
	}
	if err := s.store.LocalSessions.DeleteByVersionMismatch(ctx, nextVersion); err != nil {
		return model.LocalSession{}, err
	}
	if strings.TrimSpace(currentSessionID) != "" {
		if err := s.store.LocalSessions.Delete(ctx, currentSessionID); err != nil {
			return model.LocalSession{}, err
		}
	}
	return s.createSession(ctx, nextVersion, now)
}

func (s *Service) ValidateSession(ctx context.Context, sessionID string) (*model.LocalSession, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, ErrSessionNotFound
	}
	if err := s.store.LocalSessions.DeleteExpired(ctx, s.now()); err != nil {
		return nil, err
	}

	session, err := s.store.LocalSessions.GetBySessionID(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, ErrSessionNotFound
	}
	if session.ExpiresAt.Before(s.now()) {
		_ = s.store.LocalSessions.Delete(ctx, sessionID)
		return nil, ErrSessionNotFound
	}

	auth, err := s.store.LocalAuth.Get(ctx)
	if err != nil {
		return nil, err
	}
	if auth == nil || strings.TrimSpace(auth.PasswordHash) == "" {
		return nil, ErrPasswordNotConfigured
	}
	if session.SessionVersion != auth.SessionVersion {
		_ = s.store.LocalSessions.Delete(ctx, sessionID)
		return nil, ErrSessionNotFound
	}

	if err := s.store.LocalSessions.Touch(ctx, sessionID, s.now()); err != nil {
		return nil, err
	}
	return session, nil
}

func (s *Service) Logout(ctx context.Context, sessionID string) error {
	if strings.TrimSpace(sessionID) == "" {
		return nil
	}
	if err := s.store.LocalSessions.Delete(ctx, sessionID); err != nil {
		return err
	}
	return nil
}

func (s *Service) createSession(ctx context.Context, sessionVersion int, now time.Time) (model.LocalSession, error) {
	sessionID, err := generateSessionID()
	if err != nil {
		return model.LocalSession{}, err
	}
	session := model.LocalSession{
		SessionID:      sessionID,
		SessionVersion: sessionVersion,
		ExpiresAt:      now.Add(SessionTTL),
		LastSeenAt:     now,
	}
	if err := s.store.LocalSessions.Create(ctx, session); err != nil {
		return model.LocalSession{}, err
	}
	return session, nil
}

func validatePassword(password string) error {
	trimmed := strings.TrimSpace(password)
	if len(trimmed) < MinPasswordLength {
		return ErrPasswordTooShort
	}
	if len(trimmed) > MaxPasswordLength {
		return ErrPasswordTooLong
	}
	return nil
}

func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

func generateSessionID() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate session id: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func (l *loginLimiter) beforeLogin(now time.Time) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if now.Before(l.lockedUntil) {
		return &LoginRateLimitError{retryAfter: l.lockedUntil.Sub(now)}
	}
	return nil
}

func (l *loginLimiter) recordFailure(now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if now.After(l.lockedUntil) {
		l.lockedUntil = time.Time{}
	}
	l.failures++
	if l.failures < maxFailedLoginAttempts {
		return
	}
	l.failures = 0
	l.lockedUntil = now.Add(loginLockoutDuration)
}

func (l *loginLimiter) recordSuccess() {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.failures = 0
	l.lockedUntil = time.Time{}
}
