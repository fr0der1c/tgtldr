package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/frederic/tgtldr/app/internal/httpx"
	"github.com/frederic/tgtldr/app/internal/localauth"
)

const sessionCookieName = "tgtldr_session"

type contextKey string

const sessionIDContextKey contextKey = "localSessionID"

func (r *Router) authorizeRequest(w http.ResponseWriter, req *http.Request) (*http.Request, bool) {
	sessionID := readSessionCookie(req)
	allowAnonymous := isPublicPath(req.URL.Path)
	if sessionID == "" && allowAnonymous {
		return req, true
	}
	if sessionID == "" {
		httpx.ErrorWithCode(w, http.StatusUnauthorized, r.localized(req.Context(), "请先登录。", "Log in first."), "unauthorized", 0)
		return nil, false
	}

	session, err := r.auth.ValidateSession(req.Context(), sessionID)
	if err != nil {
		if errors.Is(err, localauth.ErrSessionNotFound) || errors.Is(err, localauth.ErrPasswordNotConfigured) {
			r.clearSessionCookie(w)
			if allowAnonymous {
				return req, true
			}
			httpx.ErrorWithCode(w, http.StatusUnauthorized, r.localized(req.Context(), "登录状态已失效，请重新登录。", "Login session expired. Log in again."), "unauthorized", 0)
			return nil, false
		}
		if allowAnonymous {
			return req, true
		}
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return nil, false
	}

	ctx := context.WithValue(req.Context(), sessionIDContextKey, session.SessionID)
	return req.WithContext(ctx), true
}

func isPublicPath(path string) bool {
	switch path {
	case "/api/health", "/api/bootstrap", "/api/auth/login", "/api/auth/logout", "/api/auth/setup-password":
		return true
	default:
		return false
	}
}

func (r *Router) setSessionCookie(w http.ResponseWriter, sessionID string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  expiresAt,
		MaxAge:   int(time.Until(expiresAt).Seconds()),
	})
}

func (r *Router) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}

func readSessionCookie(req *http.Request) string {
	cookie, err := req.Cookie(sessionCookieName)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cookie.Value)
}

func currentSessionID(ctx context.Context) string {
	value, ok := ctx.Value(sessionIDContextKey).(string)
	if !ok {
		return ""
	}
	return value
}

func (r *Router) handleLogin(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		httpx.Error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var payload struct {
		Password string `json:"password"`
	}
	if err := httpx.DecodeJSON(req, &payload); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	session, err := r.auth.Login(req.Context(), payload.Password)
	if err != nil {
		var rateLimitErr *localauth.LoginRateLimitError
		switch {
		case errors.As(err, &rateLimitErr):
			httpx.ErrorWithCode(w, http.StatusTooManyRequests, rateLimitErr.Error(), "login_rate_limited", rateLimitErr.RetryAfterSeconds())
		case errors.Is(err, localauth.ErrPasswordNotConfigured):
			httpx.Error(w, http.StatusBadRequest, r.localized(req.Context(), "请先完成首次设置向导。", "Complete the first-time setup wizard first."))
		case errors.Is(err, localauth.ErrInvalidPassword):
			httpx.ErrorWithCode(w, http.StatusUnauthorized, r.localized(req.Context(), "密码错误。", "Incorrect password."), "invalid_password", 0)
		default:
			httpx.Error(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	r.setSessionCookie(w, session.SessionID, session.ExpiresAt)
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (r *Router) handleLogout(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		httpx.Error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	_ = r.auth.Logout(req.Context(), readSessionCookie(req))
	r.clearSessionCookie(w)
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (r *Router) handleSetupPassword(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		httpx.Error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var payload struct {
		Password string `json:"password"`
	}
	if err := httpx.DecodeJSON(req, &payload); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	session, err := r.auth.SetupPassword(req.Context(), payload.Password)
	if err != nil {
		switch {
		case errors.Is(err, localauth.ErrPasswordAlreadySet):
			httpx.Error(w, http.StatusConflict, r.localized(req.Context(), "访问密码已经设置完成。", "Access password has already been set."))
		case errors.Is(err, localauth.ErrPasswordTooShort):
			httpx.Error(w, http.StatusBadRequest, r.localized(req.Context(), "访问密码至少需要 8 位。", "Access password must be at least 8 characters."))
		case errors.Is(err, localauth.ErrPasswordTooLong):
			httpx.Error(w, http.StatusBadRequest, r.localized(req.Context(), "访问密码不能超过 128 位。", "Access password cannot exceed 128 characters."))
		default:
			httpx.Error(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	r.setSessionCookie(w, session.SessionID, session.ExpiresAt)
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (r *Router) handleChangePassword(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		httpx.Error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var payload struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if err := httpx.DecodeJSON(req, &payload); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	session, err := r.auth.ChangePassword(req.Context(), payload.CurrentPassword, payload.NewPassword, currentSessionID(req.Context()))
	if err != nil {
		switch {
		case errors.Is(err, localauth.ErrInvalidPassword):
			httpx.ErrorWithCode(w, http.StatusBadRequest, r.localized(req.Context(), "当前密码不正确。", "Current password is incorrect."), "invalid_password", 0)
		case errors.Is(err, localauth.ErrPasswordTooShort):
			httpx.Error(w, http.StatusBadRequest, r.localized(req.Context(), "访问密码至少需要 8 位。", "Access password must be at least 8 characters."))
		case errors.Is(err, localauth.ErrPasswordTooLong):
			httpx.Error(w, http.StatusBadRequest, r.localized(req.Context(), "访问密码不能超过 128 位。", "Access password cannot exceed 128 characters."))
		default:
			httpx.Error(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	r.setSessionCookie(w, session.SessionID, session.ExpiresAt)
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
