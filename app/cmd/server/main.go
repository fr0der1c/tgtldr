package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/fr0der1c/tgtldr/app/internal/api"
	"github.com/fr0der1c/tgtldr/app/internal/bot"
	"github.com/fr0der1c/tgtldr/app/internal/clock"
	"github.com/fr0der1c/tgtldr/app/internal/config"
	"github.com/fr0der1c/tgtldr/app/internal/model"
	"github.com/fr0der1c/tgtldr/app/internal/scheduler"
	"github.com/fr0der1c/tgtldr/app/internal/store"
	"github.com/fr0der1c/tgtldr/app/internal/summary"
	telegramsvc "github.com/fr0der1c/tgtldr/app/internal/telegram"
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
	telegramService := telegramsvc.NewService(ctx, st, sysClock, cfg.MediaDir)
	schedulerService := scheduler.NewService(st, sysClock, summaryService, botService)
	telegramService.SetHistoryBackfillCompletionHook(func(chat model.Chat, fromDate, toDate string) {
		_ = schedulerService.RepairEmptySummariesInRange(context.Background(), chat, fromDate, toDate)
	})
	router := api.New(
		st,
		telegramService,
		schedulerService,
		botService,
		cfg.WebOrigin,
		cfg.RequestTimout,
		cfg.MediaDir,
	)

	if accounts, err := st.Auth.List(ctx); err == nil {
		for _, account := range accounts {
			if account.Status == "authorized" {
				telegramService.EnsureListener(account.ID)
			}
		}
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
