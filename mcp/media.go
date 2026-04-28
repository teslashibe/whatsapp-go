package mcp

import (
	"context"

	whatsapp "github.com/teslashibe/whatsapp-go"
	"github.com/teslashibe/mcptool"
)

type GetMediaInput struct {
	MessageID string `json:"message_id" jsonschema:"description=message ID to download media from,required"`
}

func getMedia(ctx context.Context, c *whatsapp.Client, in GetMediaInput) (any, error) {
	return c.GetMedia(ctx, in.MessageID)
}

var mediaTools = []mcptool.Tool{
	mcptool.Define[*whatsapp.Client, GetMediaInput](
		"whatsapp_get_media",
		"Download a media payload by message ID and return it as base64 (subject to host size cap)",
		"GetMedia",
		getMedia,
	),
}
