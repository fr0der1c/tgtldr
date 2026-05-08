package telegram

import (
	"context"
	"strings"

	"github.com/frederic/tgtldr/app/internal/model"
	"github.com/frederic/tgtldr/app/internal/store"
	"github.com/gotd/td/telegram"
)

func (s *Service) newClient() (*telegram.Client, model.AppSettings, error) {
	settings, err := s.store.Settings.Get(context.Background())
	if err != nil {
		return nil, model.AppSettings{}, err
	}
	if settings.TelegramAPIID == 0 || strings.TrimSpace(settings.TelegramAPIHash) == "" {
		return nil, model.AppSettings{}, ErrConfigIncomplete
	}
	client, err := s.newConfiguredClient(nil)
	if err != nil {
		return nil, model.AppSettings{}, err
	}
	return client, settings, nil
}

func (s *Service) newConfiguredClient(handler telegram.UpdateHandler) (*telegram.Client, error) {
	settings, _ := s.store.Settings.Get(context.Background())
	resolver, err := telegramProxyResolverFromEnv()
	if err != nil {
		return nil, err
	}

	options := telegram.Options{
		SessionStorage: store.NewSessionStorage(s.store.Auth),
		UpdateHandler:  handler,
		Device: telegram.DeviceConfig{
			DeviceModel:    "TGTLDR",
			SystemVersion:  "Desktop",
			AppVersion:     "Self-hosted",
			SystemLangCode: "zh",
			LangCode:       "zh",
		},
	}
	if resolver != nil {
		options.Resolver = resolver
	}
	return telegram.NewClient(settings.TelegramAPIID, settings.TelegramAPIHash, options), nil
}
