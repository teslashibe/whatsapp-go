package mcp

import (
	"context"

	whatsapp "github.com/teslashibe/whatsapp-go"
	"github.com/teslashibe/mcptool"
)

type ReactInput struct {
	JID       string `json:"jid" jsonschema:"description=chat JID containing the target message,required"`
	MessageID string `json:"message_id" jsonschema:"description=ID of the message to react to,required"`
	Emoji     string `json:"emoji" jsonschema:"description=single emoji (or empty string to remove a previous reaction),required"`
	FromMe    bool   `json:"from_me,omitempty" jsonschema:"description=true if the target message was sent by the authenticated account,default=false"`
	Confirm   bool   `json:"confirm" jsonschema:"description=must be true when the host enforces send-confirmation,required"`
}

func react(ctx context.Context, c *whatsapp.Client, in ReactInput) (any, error) {
	if err := c.React(ctx, whatsapp.ReactParams{
		JID: in.JID, MessageID: in.MessageID, Emoji: in.Emoji,
		FromMe: in.FromMe, Confirm: in.Confirm,
	}); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

var reactTools = []mcptool.Tool{
	mcptool.Define[*whatsapp.Client, ReactInput](
		"whatsapp_react",
		"Send a tapback reaction to a WhatsApp message; pass an empty emoji to remove a prior reaction",
		"React",
		react,
	),
}
