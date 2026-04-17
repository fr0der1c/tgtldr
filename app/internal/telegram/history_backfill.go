package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/frederic/tgtldr/app/internal/model"
	"github.com/gotd/td/telegram/message/peer"
	messagesquery "github.com/gotd/td/telegram/query/messages"
	"github.com/gotd/td/tg"
)

const maxHistoryBackfillFloodWait = 20 * time.Minute

type historyBackfillProgress struct {
	offsetID   int
	offsetDate int
}

type historyBackfillFloodLimitError struct {
	totalWait time.Duration
}

func (e *historyBackfillFloodLimitError) Error() string {
	minutes := int(e.totalWait.Minutes())
	if minutes < 1 {
		minutes = 1
	}
	return fmt.Sprintf("Telegram 持续限流，系统已自动等待约 %d 分钟仍未完成回补，请稍后再试。", minutes)
}

type historyBackfillStore struct {
	mu    sync.RWMutex
	tasks map[string]model.HistoryBackfillTask
}

func newHistoryBackfillStore() *historyBackfillStore {
	return &historyBackfillStore{tasks: map[string]model.HistoryBackfillTask{}}
}

func (s *Service) StartHistoryBackfill(chat model.Chat, fromDate, toDate string) (model.HistoryBackfillTask, error) {
	settings, err := s.store.Settings.Get(context.Background())
	if err != nil {
		return model.HistoryBackfillTask{}, err
	}

	timezone := strings.TrimSpace(settings.DefaultTimezone)
	if timezone == "" {
		timezone = time.Local.String()
	}
	start, endExclusive, err := parseHistoryRange(fromDate, toDate, timezone)
	if err != nil {
		return model.HistoryBackfillTask{}, err
	}

	task := model.HistoryBackfillTask{
		ID:        strconv.FormatInt(s.clock.Now().UnixNano(), 36),
		ChatID:    chat.ID,
		ChatTitle: chat.Title,
		FromDate:  fromDate,
		ToDate:    toDate,
		Status:    model.HistoryBackfillStatusPending,
		CreatedAt: s.clock.Now(),
		UpdatedAt: s.clock.Now(),
	}
	s.saveHistoryTask(task)

	go s.runHistoryBackfill(task.ID, chat, start, endExclusive)
	return task, nil
}

func (s *Service) GetHistoryBackfillTask(taskID string) (model.HistoryBackfillTask, error) {
	task, ok := s.historyBackfills.get(taskID)
	if !ok {
		return model.HistoryBackfillTask{}, fmt.Errorf("history backfill task %s not found", taskID)
	}
	return task, nil
}

func (s *Service) runHistoryBackfill(taskID string, chat model.Chat, start, endExclusive time.Time) {
	s.updateHistoryTask(taskID, func(task *model.HistoryBackfillTask) {
		task.Status = model.HistoryBackfillStatusRunning
		task.UpdatedAt = s.clock.Now()
		task.ErrorMessage = ""
	})

	processedIDs := map[int]struct{}{}
	processedCount := 0
	progress := historyBackfillProgress{}
	totalWait := time.Duration(0)

	for attempt := 1; ; attempt++ {
		count, nextProgress, err := s.runHistoryBackfillAttempt(chat, start, endExclusive, processedIDs, progress)
		processedCount = count
		progress = nextProgress
		if err == nil {
			now := s.clock.Now()
			s.updateHistoryTask(taskID, func(task *model.HistoryBackfillTask) {
				task.Status = model.HistoryBackfillStatusSucceeded
				task.ImportedCount = processedCount
				task.UpdatedAt = now
				task.CompletedAt = &now
				task.ErrorMessage = ""
			})
			return
		}

		floodErr, ok := asHistoryBackfillFloodWait(err)
		if !ok {
			s.failHistoryTask(taskID, err)
			return
		}
		if totalWait+floodErr.Wait > maxHistoryBackfillFloodWait {
			s.failHistoryTask(taskID, &historyBackfillFloodLimitError{totalWait: totalWait + floodErr.Wait})
			return
		}

		totalWait += floodErr.Wait
		s.waitForHistoryBackfillRetry(taskID, floodErr, processedCount)
	}
}

func (s *Service) runHistoryBackfillAttempt(
	chat model.Chat,
	start time.Time,
	endExclusive time.Time,
	processedIDs map[int]struct{},
	progress historyBackfillProgress,
) (int, historyBackfillProgress, error) {
	client, _, err := s.newClient()
	if err != nil {
		return len(processedIDs), progress, err
	}

	count := len(processedIDs)
	current := progress
	err = client.Run(s.root, func(ctx context.Context) error {
		status, err := client.Auth().Status(ctx)
		if err != nil {
			return err
		}
		if !status.Authorized {
			return fmt.Errorf("telegram session not authorized")
		}

		inputPeer, err := inputPeerForChat(chat)
		if err != nil {
			return err
		}

		query := messagesquery.NewQueryBuilder(client.API()).
			GetHistory(inputPeer).
			BatchSize(100).
			OffsetID(current.offsetID).
			OffsetDate(current.offsetDate)
		iter := query.Iter()
		for iter.Next(ctx) {
			elem := iter.Value()
			current.offsetID = elem.Msg.GetID()
			current.offsetDate = elem.Msg.GetDate()

			messageTime := time.Unix(int64(elem.Msg.GetDate()), 0).UTC()
			if !messageTime.Before(endExclusive) {
				continue
			}
			if messageTime.Before(start) {
				break
			}

			item, ok := historyElemToMessage(chat, elem)
			if !ok {
				continue
			}
			if err := s.store.Messages.Upsert(ctx, item); err != nil {
				return err
			}
			if _, exists := processedIDs[item.TelegramMessageID]; !exists {
				processedIDs[item.TelegramMessageID] = struct{}{}
				count++
			}
		}
		err = iter.Err()
		if err != nil {
			return wrapTelegramError(err)
		}
		return nil
	})
	if err != nil {
		return count, current, err
	}
	return count, current, nil
}

func (s *Service) waitForHistoryBackfillRetry(taskID string, floodErr *FloodWaitError, processedCount int) {
	retryLabel := fmt.Sprintf("Telegram 正在限流，系统会在 %d 秒后自动继续。", floodErr.RetryAfterSeconds())
	s.updateHistoryTask(taskID, func(task *model.HistoryBackfillTask) {
		task.Status = model.HistoryBackfillStatusRunning
		task.ImportedCount = processedCount
		task.ErrorMessage = retryLabel
		task.UpdatedAt = s.clock.Now()
	})

	timer := time.NewTimer(floodErr.Wait)
	defer timer.Stop()

	select {
	case <-timer.C:
	case <-s.root.Done():
	}
}

func asHistoryBackfillFloodWait(err error) (*FloodWaitError, bool) {
	var floodErr *FloodWaitError
	if !errors.As(err, &floodErr) {
		return nil, false
	}
	return floodErr, true
}

func (s *Service) failHistoryTask(taskID string, err error) {
	now := s.clock.Now()
	s.updateHistoryTask(taskID, func(task *model.HistoryBackfillTask) {
		task.Status = model.HistoryBackfillStatusFailed
		task.ErrorMessage = err.Error()
		task.UpdatedAt = now
		task.CompletedAt = &now
	})
}

func (s *Service) saveHistoryTask(task model.HistoryBackfillTask) {
	s.historyBackfills.save(task)
}

func (s *Service) updateHistoryTask(taskID string, apply func(*model.HistoryBackfillTask)) {
	s.historyBackfills.update(taskID, apply)
}

func parseHistoryRange(fromDate, toDate, timezone string) (time.Time, time.Time, error) {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("load location %s: %w", timezone, err)
	}

	start, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(fromDate), location)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("parse from date %s: %w", fromDate, err)
	}
	end, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(toDate), location)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("parse to date %s: %w", toDate, err)
	}
	if end.Before(start) {
		return time.Time{}, time.Time{}, fmt.Errorf("结束日期不能早于开始日期")
	}
	return start.UTC(), end.Add(24 * time.Hour).UTC(), nil
}

func inputPeerForChat(chat model.Chat) (tg.InputPeerClass, error) {
	switch chat.ChatType {
	case "group":
		return &tg.InputPeerChat{ChatID: chat.TelegramChatID}, nil
	case "supergroup":
		return &tg.InputPeerChannel{
			ChannelID:  chat.TelegramChatID,
			AccessHash: chat.TelegramAccess,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported chat type %s", chat.ChatType)
	}
}

func historyElemToMessage(chat model.Chat, elem messagesquery.Elem) (model.Message, bool) {
	msg, ok := elem.Msg.(*tg.Message)
	if !ok {
		return model.Message{}, false
	}

	payload, _ := json.Marshal(msg)
	senderID, senderName, senderUsername, senderIsBot := resolveSenderFromPeerEntities(msg, elem.Entities)
	return model.Message{
		ChatID:            chat.ID,
		TelegramMessageID: msg.ID,
		TelegramSenderID:  senderID,
		SenderName:        senderName,
		SenderUsername:    senderUsername,
		SenderIsBot:       senderIsBot,
		TextContent:       msg.Message,
		Caption:           extractCaption(msg),
		MessageType:       classifyMessage(msg),
		MediaKind:         mediaKind(msg),
		ReplyToMessageID:  replyToID(msg),
		MessageTime:       time.Unix(int64(msg.Date), 0).UTC(),
		RawJSON:           string(payload),
	}, true
}

func resolveSenderFromPeerEntities(msg *tg.Message, entities peer.Entities) (int64, string, string, bool) {
	switch from := msg.FromID.(type) {
	case *tg.PeerUser:
		user, ok := entities.User(from.UserID)
		if !ok {
			return from.UserID, "User " + int64String(from.UserID), "", false
		}
		name := strings.TrimSpace(strings.TrimSpace(user.FirstName) + " " + strings.TrimSpace(user.LastName))
		if name == "" {
			name = user.Username
		}
		if name == "" {
			name = "User " + int64String(user.ID)
		}
		return user.ID, name, user.Username, user.Bot
	case *tg.PeerChannel:
		channel, ok := entities.Channel(from.ChannelID)
		if ok {
			return channel.ID, channel.Title, channel.Username, false
		}
		return from.ChannelID, "Channel " + int64String(from.ChannelID), "", false
	case *tg.PeerChat:
		groupChat, ok := entities.Chat(from.ChatID)
		if ok {
			return groupChat.ID, groupChat.Title, "", false
		}
		return from.ChatID, "Chat " + int64String(from.ChatID), "", false
	default:
		return 0, "Unknown", "", false
	}
}

func (s *historyBackfillStore) get(taskID string) (model.HistoryBackfillTask, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, ok := s.tasks[taskID]
	return task, ok
}

func (s *historyBackfillStore) save(task model.HistoryBackfillTask) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasks[task.ID] = task
}

func (s *historyBackfillStore) update(taskID string, apply func(*model.HistoryBackfillTask)) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[taskID]
	if !ok {
		return
	}
	apply(&task)
	s.tasks[taskID] = task
}
