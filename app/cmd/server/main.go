package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/frederic/tgtldr/app/internal/api"
	"github.com/frederic/tgtldr/app/internal/bot"
	"github.com/frederic/tgtldr/app/internal/clock"
	"github.com/frederic/tgtldr/app/internal/config"
	"github.com/frederic/tgtldr/app/internal/model"
	"github.com/frederic/tgtldr/app/internal/scheduler"
	"github.com/frederic/tgtldr/app/internal/store"
	"github.com/frederic/tgtldr/app/internal/summary"
	telegramsvc "github.com/frederic/tgtldr/app/internal/telegram"
	"golang.org/x/sync/errgroup"
)

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	st, err := store.Open(ctx, cfg)
	if err != nil {
		return err
	}
	defer st.Close()

	if err := store.RunMigrations(ctx, st); err != nil {
		return err
	}

	sysClock := clock.System{}
	botService := bot.New()
	summaryService := summary.NewService(st, sysClock, cfg.OpenAITimeout)
	telegramService := telegramsvc.NewService(ctx, st, sysClock)
	schedulerService := scheduler.NewService(st, sysClock, summaryService, botService)
	telegramService.SetHistoryBackfillCompletionHook(func(chat model.Chat, fromDate, toDate string) {
		_ = schedulerService.RepairEmptySummariesInRange(context.Background(), chat, fromDate, toDate)
	})
	telegramService.SetAlertHook(func(chat model.Chat, msg model.Message, keyword string) {
		bgCtx := context.Background()
		settings, err := st.Settings.Get(bgCtx)
		if err != nil || !settings.BotEnabled || strings.TrimSpace(settings.BotToken) == "" {
			return
		}
		text := fmt.Sprintf("🔔 **%s** 关键词提醒：`%s`\n\n%s", chat.Title, keyword, msg.TextContent)
		_ = botService.SendMessageWithLanguage(bgCtx, settings.BotToken, settings.BotTargetChatID, text, settings.Language)
	})
	router := api.New(
		st,
		telegramService,
		schedulerService,
		botService,
		cfg.WebOrigin,
		cfg.RequestTimout,
	)

	if auth, err := telegramService.BootstrapAuth(ctx); err == nil && auth != nil && auth.Status == "authorized" {
		telegramService.EnsureListener()
	}

	server := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: router.Handler(),
	}

	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		if err := schedulerService.Run(groupCtx); err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
		return nil
	})
	group.Go(func() error {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})
	group.Go(func() error {
		<-groupCtx.Done()
		telegramService.StopListener()
		return server.Shutdown(context.Background())
	})

	return group.Wait()
}
