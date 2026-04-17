package bot

import (
	"reflect"
	"testing"
)

func TestMatchTargetChatCandidates(t *testing.T) {
	t.Parallel()

	updates := []botUpdate{
		{
			UpdateID: 10,
			Message: &botMessage{
				Date: 100,
				From: &botUser{ID: 42},
				Chat: botChat{
					ID:   1001,
					Type: "private",
				},
			},
		},
		{
			UpdateID: 11,
			Message: &botMessage{
				Date: 101,
				From: &botUser{ID: 7},
				Chat: botChat{
					ID:    -1002,
					Type:  "supergroup",
					Title: "别人的群",
				},
			},
		},
		{
			UpdateID: 12,
			Message: &botMessage{
				Date: 110,
				From: &botUser{ID: 42},
				Chat: botChat{
					ID:        -1002,
					Type:      "supergroup",
					Title:     "团队群",
					Username:  "team_group",
					FirstName: "ignored",
				},
			},
		},
		{
			UpdateID: 13,
			Message: &botMessage{
				Date: 109,
				From: &botUser{ID: 42},
				Chat: botChat{
					ID:    -1002,
					Type:  "supergroup",
					Title: "旧团队群",
				},
			},
		},
		{
			UpdateID: 14,
			Message: &botMessage{
				Date: 108,
				From: &botUser{ID: 42},
				Chat: botChat{
					ID:        1003,
					Type:      "private",
					FirstName: "Frederic",
					LastName:  "Zhang",
					Username:  "frederic",
				},
			},
		},
		{
			UpdateID: 15,
			Message:  nil,
		},
	}

	got := matchTargetChatCandidates(updates, 42)
	want := []TargetChatCandidate{
		{
			ChatID:   "-1002",
			ChatType: "supergroup",
			Title:    "团队群",
			Username: "team_group",
		},
		{
			ChatID:   "1003",
			ChatType: "private",
			Title:    "Frederic Zhang",
			Username: "frederic",
		},
		{
			ChatID:   "1001",
			ChatType: "private",
			Title:    "与 Bot 的私聊",
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("matchTargetChatCandidates() mismatch:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestMatchTargetChatCandidatesZeroUserID(t *testing.T) {
	t.Parallel()

	got := matchTargetChatCandidates([]botUpdate{{
		UpdateID: 1,
		Message: &botMessage{
			Date: 1,
			From: &botUser{ID: 42},
			Chat: botChat{ID: 1001, Type: "private"},
		},
	}}, 0)

	if len(got) != 0 {
		t.Fatalf("expected no candidates, got %#v", got)
	}
}
