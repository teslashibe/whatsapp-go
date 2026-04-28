package whatsapp

import (
	"context"
	"strings"
	"time"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types/events"
)

// handleEvent is the single event handler whatsmeow calls. We dispatch
// to per-type helpers and keep individual handlers cheap (the event loop
// is shared across all events).
func (c *Client) handleEvent(raw any) {
	switch e := raw.(type) {
	case *events.Message:
		c.onMessage(e)
	case *events.Receipt:
		// receipts could be persisted; left as a future enhancement.
	case *events.Connected:
		// no-op; Status() reads IsConnected() directly.
	case *events.Disconnected:
		c.mu.Lock()
		c.connected = false
		c.mu.Unlock()
	case *events.LoggedOut:
		c.mu.Lock()
		c.connected = false
		c.mu.Unlock()
	}
}

// onMessage normalises a whatsmeow message event into the local Message
// model and persists it.
func (c *Client) onMessage(e *events.Message) {
	if e == nil || e.Message == nil {
		return
	}
	body, mediaKind, mediaName, hasMedia := extractContent(e.Message)
	m := Message{
		MessageID:  e.Info.ID,
		ChatJID:    e.Info.Chat.String(),
		SenderJID:  e.Info.Sender.String(),
		IsFromMe:   e.Info.IsFromMe,
		IsGroup:    e.Info.IsGroup,
		Body:       body,
		MediaKind:  mediaKind,
		MediaName:  mediaName,
		QuotedID:   extractQuotedID(e.Message),
		Timestamp:  e.Info.Timestamp,
		SenderName: e.Info.PushName,
		HasMedia:   hasMedia,
	}
	if m.Timestamp.IsZero() {
		m.Timestamp = time.Now()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = c.upsertMessage(ctx, m)
	if m.SenderName != "" && !m.IsGroup {
		c.updateChatDisplayName(ctx, m.ChatJID, m.SenderName)
	}
}

// extractContent pulls the user-visible text and media metadata out of a
// waE2E.Message proto across the many shapes WhatsApp uses.
func extractContent(msg *waE2E.Message) (body string, kind MediaKind, name string, hasMedia bool) {
	if msg == nil {
		return
	}
	if c := msg.GetConversation(); c != "" {
		body = c
	}
	if ext := msg.GetExtendedTextMessage(); ext != nil && body == "" {
		body = ext.GetText()
	}
	if im := msg.GetImageMessage(); im != nil {
		kind, hasMedia = MediaImage, true
		if body == "" {
			body = im.GetCaption()
		}
		name = im.GetCaption()
	}
	if vm := msg.GetVideoMessage(); vm != nil {
		kind, hasMedia = MediaVideo, true
		if body == "" {
			body = vm.GetCaption()
		}
	}
	if am := msg.GetAudioMessage(); am != nil {
		kind, hasMedia = MediaAudio, true
	}
	if dm := msg.GetDocumentMessage(); dm != nil {
		kind, hasMedia = MediaDocument, true
		name = dm.GetFileName()
		if body == "" {
			body = dm.GetCaption()
		}
	}
	if sm := msg.GetStickerMessage(); sm != nil {
		kind, hasMedia = MediaSticker, true
	}
	body = strings.TrimSpace(body)
	return
}

func extractQuotedID(msg *waE2E.Message) string {
	if msg == nil {
		return ""
	}
	if ext := msg.GetExtendedTextMessage(); ext != nil {
		if ctx := ext.GetContextInfo(); ctx != nil {
			return ctx.GetStanzaID()
		}
	}
	return ""
}
