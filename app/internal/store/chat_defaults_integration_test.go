package store

import (
	"testing"

	"github.com/fr0der1c/tgtldr/app/internal/model"
)

// TestChatDefaultsPersistence 验证默认配置只作用于新群组，并可一键覆盖全部群组。
func TestChatDefaultsPersistence(t *testing.T) {
	st := recoveryTestStore(t)
	ctx := t.Context()
	defaults, err := st.Settings.GetChatDefaults(ctx)
	if err != nil || defaults != model.DefaultChatDefaults() {
		t.Fatalf("initial defaults: %+v %v", defaults, err)
	}

	existing, err := st.Chats.EnsureExists(ctx, model.Chat{TelegramChatID: 1, Title: "old", ChatType: "group"}, defaults)
	if err != nil {
		t.Fatal(err)
	}

	custom := model.ChatDefaults{DeliveryMode: model.DeliveryModeBot, SummaryTimeLocal: "07:15", ModelOverride: "custom-model", KeepBotMessages: false}
	if err = st.Settings.SaveChatDefaults(ctx, custom); err != nil {
		t.Fatal(err)
	}
	if defaults, err = st.Settings.GetChatDefaults(ctx); err != nil || defaults != custom {
		t.Fatalf("saved defaults: %+v %v", defaults, err)
	}

	created, err := st.Chats.EnsureExists(ctx, model.Chat{TelegramChatID: 2, Title: "new", ChatType: "group"}, defaults)
	if err != nil || created.DeliveryMode != model.DeliveryModeBot || created.SummaryTimeLocal != "07:15" || created.ModelOverride != "custom-model" || created.KeepBotMessages {
		t.Fatalf("new chat: %+v %v", created, err)
	}
	resynced, err := st.Chats.EnsureExists(ctx, model.Chat{TelegramChatID: 1, Title: "renamed", ChatType: "group"}, defaults)
	if err != nil || resynced.Title != "renamed" || resynced.DeliveryMode != existing.DeliveryMode || resynced.SummaryTimeLocal != existing.SummaryTimeLocal {
		t.Fatalf("existing chat must keep its settings: %+v %v", resynced, err)
	}

	updated, err := st.Chats.ApplyDefaults(ctx, defaults)
	if err != nil || updated != 2 {
		t.Fatalf("apply: %d %v", updated, err)
	}
	applied, err := st.Chats.GetByTelegramID(ctx, 1)
	if err != nil || applied.DeliveryMode != model.DeliveryModeBot || applied.SummaryTimeLocal != "07:15" || applied.KeepBotMessages {
		t.Fatalf("applied: %+v %v", applied, err)
	}
}
