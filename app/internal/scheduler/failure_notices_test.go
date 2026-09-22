package scheduler

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/bot"
	"github.com/fr0der1c/tgtldr/app/internal/model"
	"github.com/fr0der1c/tgtldr/app/internal/store"
)

// TestFailureDeliveryError 验证投递原因可定位，且不会把请求 URL 或 Token 写入通知历史。
func TestFailureDeliveryError(t *testing.T) {
	cases := []struct {
		err  error
		code string
	}{
		{nil, ""},
		{&bot.DeliveryError{StatusCode: 401, Description: "secret-token"}, "auth"},
		{&bot.DeliveryError{StatusCode: 403, Description: "secret-token"}, "permission"},
		{&bot.DeliveryError{StatusCode: 429, Description: "secret-token"}, "rate_limit"},
		{&bot.DeliveryError{StatusCode: 503, Description: "secret-token"}, "upstream"},
		{context.DeadlineExceeded, "timeout"},
		{errors.New("https://api.telegram.org/botsecret-token/sendMessage"), "delivery_failed"},
	}
	for _, tt := range cases {
		if got := failureDeliveryError(tt.err); got != tt.code {
			t.Fatalf("got %q, want %q", got, tt.code)
		}
	}
}

// TestFailureMessage 验证等待重试、合并失败和敏感响应脱敏。
func TestFailureMessage(t *testing.T) {
	next := time.Now().Add(time.Minute)
	cases := []struct {
		name   string
		states []store.FailureState
		ready  bool
	}{
		{"running", []store.FailureState{{Status: "failed"}, {Status: "running"}}, false},
		{"retry pending", []store.FailureState{{Status: "failed", NextRetryAt: &next, RetryCount: 3}}, false},
		{"retry exhausted", []store.FailureState{{Status: "failed", NextRetryAt: &next, RetryCount: 4}}, true},
		{"permanent failure", []store.FailureState{{Status: "failed"}}, true},
		{"success", []store.FailureState{{Status: "succeeded"}}, false},
		{"digest retry pending", []store.FailureState{{Status: "failed"}, {Kind: "digest", Status: "failed", NextRetryAt: &next}}, false},
		{"digest only", []store.FailureState{{Status: "succeeded"}, {Kind: "digest", Status: "failed"}}, true},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			_, ready := failureMessage("2026-09-09", tt.states, 4, model.LanguageZhCN, "", true)
			if ready != tt.ready {
				t.Fatalf("ready=%v", ready)
			}
		})
	}
	states := []store.FailureState{{Status: "failed", Error: `model not found secret-token`}, {Status: "failed", Error: `status 503: private chat body`}}
	for _, language := range []model.Language{model.LanguageZhCN, model.LanguageEN} {
		message, ready := failureMessage("2026-09-09", states, 4, language, "https://example.test", true)
		if !ready || !strings.Contains(message, "2 ") || !strings.Contains(message, "https://example.test/dashboard/summaries") {
			t.Fatalf("message=%s", message)
		}
		if strings.Contains(message, "secret-token") || strings.Contains(message, "private chat body") {
			t.Fatal("raw upstream error leaked")
		}
	}
	message, _ := failureMessage("2026-09-09", states, 4, model.LanguageEN, "", false)
	if strings.Contains(message, "Daily Digest") {
		t.Fatal("per-chat notification refers to a disabled digest")
	}
	message, _ = failureMessage("2026-09-09", []store.FailureState{{Status: "failed"}}, 4, model.LanguageEN, "", true)
	if strings.Contains(message, "Daily Digest") {
		t.Fatal("dashboard-only chat incorrectly affects digest")
	}
	message, _ = failureMessage("2026-09-09", []store.FailureState{{Status: "failed", InDigest: true}}, 4, model.LanguageEN, "", true)
	if !strings.Contains(message, "Daily Digest") {
		t.Fatal("missing digest omission notice")
	}
}

// TestRecoveryRound 验证两条并发上限、失败不阻塞和总览最后执行。
func TestRecoveryRound(t *testing.T) {
	items := []store.RecoveryItem{{Kind: "digest", ID: 10, Status: "queued"}, {Kind: "summary", ID: 1, Status: "failed"}, {Kind: "summary", ID: 2, Status: "queued"}, {Kind: "summary", ID: 3, Status: "queued"}, {Kind: "summary", ID: 4, Status: "queued"}}
	got := recoveryRound(items)
	if len(got) != 2 || got[0].ID != 2 || got[1].ID != 3 {
		t.Fatalf("unexpected selection: %+v", got)
	}
	for i := range items {
		if items[i].Kind == "summary" {
			items[i].Status = "succeeded"
		}
	}
	got = recoveryRound(items)
	if len(got) != 1 || got[0].Kind != "digest" {
		t.Fatalf("digest not selected: %+v", got)
	}
}
