package im_test

import (
	"testing"

	"github.com/Foolyou/acp-assistant/internal/im"
	"github.com/Foolyou/acp-assistant/internal/model"
)

func TestFeishuPrivateMessageNormalizationRejectsGroupChat(t *testing.T) {
	msg, decision, err := im.NormalizeFeishuEvent("main", []byte(`{
		"event": {
			"message": {
				"message_id": "mid-1",
				"chat_id": "chat-1",
				"chat_type": "p2p",
				"create_time": "1710000000000",
				"message_type": "text",
				"content": "{\"text\":\"hello\"}"
			},
			"sender": {"sender_id": {"open_id": "ou-user"}}
		}
	}`))
	if err != nil {
		t.Fatalf("normalize private: %v", err)
	}
	if decision != im.DecisionAccepted || msg.Platform != model.PlatformFeishu || msg.Text != "hello" || msg.PlatformUserID != "ou-user" {
		t.Fatalf("unexpected normalized message decision=%s msg=%#v", decision, msg)
	}

	_, decision, err = im.NormalizeFeishuEvent("main", []byte(`{
		"event": {
			"message": {"message_id":"mid-2","chat_id":"chat-2","chat_type":"group","message_type":"text","content":"{\"text\":\"no\"}"},
			"sender": {"sender_id": {"open_id": "ou-user"}}
		}
	}`))
	if err != nil {
		t.Fatalf("group event should be a rejection decision, not a parser error: %v", err)
	}
	if decision != im.DecisionRejectedNonPrivate {
		t.Fatalf("unexpected decision for group event: %s", decision)
	}
}

func TestQQBotC2CNormalizationRejectsGuildEvents(t *testing.T) {
	msg, decision, err := im.NormalizeQQBotEvent("main", []byte(`{
		"t": "C2C_MESSAGE_CREATE",
		"id": "event-1",
		"d": {
			"id": "msg-1",
			"author": {"id": "qq-user"},
			"content": "hello qq",
			"timestamp": "2026-05-20T10:00:00Z"
		}
	}`))
	if err != nil {
		t.Fatalf("normalize c2c: %v", err)
	}
	if decision != im.DecisionAccepted || msg.Platform != model.PlatformQQBot || msg.Text != "hello qq" || msg.PrivateChannelID != "qq-user" {
		t.Fatalf("unexpected normalized message decision=%s msg=%#v", decision, msg)
	}

	_, decision, err = im.NormalizeQQBotEvent("main", []byte(`{
		"t": "AT_MESSAGE_CREATE",
		"id": "event-2",
		"d": {"id": "msg-2", "guild_id": "guild", "channel_id": "channel", "content": "no"}
	}`))
	if err != nil {
		t.Fatalf("guild event should be a rejection decision, not a parser error: %v", err)
	}
	if decision != im.DecisionRejectedNonPrivate {
		t.Fatalf("unexpected decision for guild event: %s", decision)
	}
}
