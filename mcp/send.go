package mcp

import (
	"context"

	whatsapp "github.com/teslashibe/whatsapp-go"
	"github.com/teslashibe/mcptool"
)

type SendMessageInput struct {
	JID      string `json:"jid" jsonschema:"description=recipient JID (user or group),required"`
	Body     string `json:"body" jsonschema:"description=plain-text message body,required"`
	QuotedID string `json:"quoted_message_id,omitempty" jsonschema:"description=optional message ID to reply to"`
	Confirm  bool   `json:"confirm" jsonschema:"description=must be true when the host enforces send-confirmation,required"`
}

func sendMessage(ctx context.Context, c *whatsapp.Client, in SendMessageInput) (any, error) {
	if err := c.SendMessage(ctx, whatsapp.SendParams{
		JID: in.JID, Body: in.Body, QuotedID: in.QuotedID, Confirm: in.Confirm,
	}); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

type SendMediaInput struct {
	JID      string `json:"jid" jsonschema:"description=recipient JID (user or group),required"`
	FilePath string `json:"file_path" jsonschema:"description=absolute path to a local file to send,required"`
	Kind     string `json:"kind,omitempty" jsonschema:"description=force a media kind; auto-detected when omitted,enum=image,enum=video,enum=audio,enum=document,enum=sticker"`
	Caption  string `json:"caption,omitempty" jsonschema:"description=optional caption (image/video/document only)"`
	MIMEType string `json:"mime_type,omitempty" jsonschema:"description=optional MIME type override; auto-detected from file content when omitted"`
	Confirm  bool   `json:"confirm" jsonschema:"description=must be true when the host enforces send-confirmation,required"`
}

func sendMedia(ctx context.Context, c *whatsapp.Client, in SendMediaInput) (any, error) {
	if err := c.SendMedia(ctx, whatsapp.SendMediaParams{
		JID:      in.JID,
		FilePath: in.FilePath,
		Kind:     whatsapp.MediaKind(in.Kind),
		Caption:  in.Caption,
		MIMEType: in.MIMEType,
		Confirm:  in.Confirm,
	}); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

var sendTools = []mcptool.Tool{
	mcptool.Define[*whatsapp.Client, SendMessageInput](
		"whatsapp_send_message",
		"Send a plain-text WhatsApp message to a user or group JID, optionally as a reply",
		"SendMessage",
		sendMessage,
	),
	mcptool.Define[*whatsapp.Client, SendMediaInput](
		"whatsapp_send_media",
		"Upload and send a media file (image/video/audio/document/sticker) to a WhatsApp chat",
		"SendMedia",
		sendMedia,
	),
}
