package store

import "testing"

// TestChatAlertSettingsPersistence 验证群组提醒配置可保存并从各查询中读回。
func TestChatAlertSettingsPersistence(t *testing.T) {
	st := recoveryTestStore(t)
	ctx := t.Context()
	if _, err := st.Pool.Exec(ctx, `insert into chats(id,telegram_chat_id,title,summary_enabled,delivery_mode) overriding system value values(1,42,'test',true,'dashboard')`); err != nil {
		t.Fatal(err)
	}
	chat, err := st.Chats.GetByID(ctx, 1)
	if err != nil || chat.AlertEnabled || len(chat.AlertKeywords) != 0 {
		t.Fatalf("defaults: %+v %v", chat, err)
	}

	chat.AlertEnabled = true
	chat.AlertKeywords = []string{"airdrop", "空投"}
	saved, err := st.Chats.Save(ctx, chat)
	if err != nil || !saved.AlertEnabled || len(saved.AlertKeywords) != 2 {
		t.Fatalf("save: %+v %v", saved, err)
	}

	byTelegramID, err := st.Chats.GetByTelegramID(ctx, 42)
	if err != nil || !byTelegramID.AlertEnabled || byTelegramID.AlertKeywords[1] != "空投" {
		t.Fatalf("get by telegram id: %+v %v", byTelegramID, err)
	}
	listed, err := st.Chats.List(ctx)
	if err != nil || len(listed) != 1 || !listed[0].AlertEnabled {
		t.Fatalf("list: %+v %v", listed, err)
	}
}
