package mcp

import (
	"context"

	whatsapp "github.com/teslashibe/whatsapp-go"
	"github.com/teslashibe/mcptool"
)

type EmptyInput struct{}

func connect(ctx context.Context, c *whatsapp.Client, _ EmptyInput) (any, error) {
	if err := c.Connect(ctx); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

func disconnect(ctx context.Context, c *whatsapp.Client, _ EmptyInput) (any, error) {
	if err := c.Disconnect(ctx); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

func pairQR(ctx context.Context, c *whatsapp.Client, _ EmptyInput) (any, error) {
	return c.PairQR(ctx)
}

var authTools = []mcptool.Tool{
	mcptool.Define[*whatsapp.Client, EmptyInput](
		"whatsapp_connect",
		"Bring the WhatsApp client online; returns ErrPairingRequired when no session exists yet",
		"Connect",
		connect,
	),
	mcptool.Define[*whatsapp.Client, EmptyInput](
		"whatsapp_disconnect",
		"Tear down the WhatsApp websocket without forgetting the pairing",
		"Disconnect",
		disconnect,
	),
	mcptool.Define[*whatsapp.Client, EmptyInput](
		"whatsapp_pair_qr",
		"Begin QR pairing and return a code (with ASCII rendering) for the user to scan in the WhatsApp app",
		"PairQR",
		pairQR,
	),
}
