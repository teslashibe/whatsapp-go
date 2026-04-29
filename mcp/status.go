package mcp

import (
	"context"

	whatsapp "github.com/teslashibe/whatsapp-go"
	"github.com/teslashibe/mcptool"
)

type StatusInput struct{}

func status(ctx context.Context, c *whatsapp.Client, _ StatusInput) (any, error) {
	return c.Status(ctx)
}

var statusTools = []mcptool.Tool{
	mcptool.Define[*whatsapp.Client, StatusInput](
		"whatsapp_status",
		"Report pairing, connection, login, and local message-log readiness for the WhatsApp client",
		"Status",
		status,
	),
}
