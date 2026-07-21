package api

import (
	"testing"

	"github.com/fr0der1c/tgtldr/app/internal/model"
	"github.com/fr0der1c/tgtldr/app/internal/store"
)

// TestNewChatMessageSearchResponse 验证全局搜索所需的群组与时区信息会返回给前端。
func TestNewChatMessageSearchResponse(t *testing.T) {
	response := newChatMessageSearchResponse(store.MessageSearchResult{
		Items: []store.MessageSearchItem{{
			Message:   model.Message{ID: 12, ChatID: 7},
			ChatTitle: "测试群组",
			LocalDate: "2026-07-21",
		}},
		Total: 1, Page: 1, PageSize: 50,
	}, "Asia/Shanghai")

	if response.Timezone != "Asia/Shanghai" {
		t.Fatalf("unexpected timezone: %q", response.Timezone)
	}
	if len(response.Items) != 1 || response.Items[0].ChatID != 7 {
		t.Fatalf("unexpected items: %#v", response.Items)
	}
	if response.Items[0].ChatTitle != "测试群组" || response.Items[0].LocalDate != "2026-07-21" {
		t.Fatalf("unexpected search metadata: %#v", response.Items[0])
	}
}
