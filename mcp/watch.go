package mcp

import (
	"context"

	whatsapp "github.com/teslashibe/whatsapp-go"
	"github.com/teslashibe/mcptool"
)

type WatchInput struct {
	SinceID int64  `json:"since_id,omitempty" jsonschema:"description=cursor from a previous call (omit or 0 to bootstrap a cursor without fetching messages),minimum=0,default=0"`
	JID     string `json:"jid,omitempty" jsonschema:"description=optional chat JID filter"`
	Limit   int    `json:"limit,omitempty" jsonschema:"description=max messages per call,minimum=1,maximum=500,default=100"`
}

func watch(ctx context.Context, c *whatsapp.Client, in WatchInput) (any, error) {
	return c.Watch(ctx, whatsapp.WatchParams{
		SinceID: in.SinceID, JID: in.JID, Limit: in.Limit,
	})
}

var watchTools = []mcptool.Tool{
	mcptool.Define[*whatsapp.Client, WatchInput](
		"whatsapp_watch",
		"Poll for new WhatsApp messages with rowid > since_id; returns new messages plus a cursor for the next call",
		"Watch",
		watch,
	),
}
