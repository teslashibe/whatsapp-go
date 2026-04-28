package whatsapp

import (
	"context"
	"strings"
	"time"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types/events"
)

// makeHandler binds an event handler to a specific epoch so stale
// Disconnected/LoggedOut events from a previous wmeowClient don't
// mutate the state of a freshly-installed one.
func (c *Client) makeHandler(epoch uint64) func(any) {
	return func(raw any) {
		c.mu.RLock()
		current := c.wmeowEpoch
		c.mu.RUnlock()
		if current != epoch {
			// Event from a stale client; ignore.
			return
		}
		c.handleEvent(raw)
	}
}

// handleEvent dispatches a single whatsmeow event.
func (c *Client) handleEvent(raw any) {
	switch e := raw.(type) {
	case *events.Message:
		c.onMessage(e)
	case *events.Receipt:
		// receipts could be persisted; left as a future enhancement.
	case *events.Connected:
		c.mu.Lock()
		c.connected = true
		c.mu.Unlock()
	case *events.Disconnected:
		c.mu.Lock()
		c.connected = false
		c.mu.Unlock()
	case *events.LoggedOut:
		c.logger.Warnf("WhatsApp logged us out (reason: %v)", e.Reason)
		c.mu.Lock()
		c.connected = false
		c.mu.Unlock()
	}
}

// onMessage normalises a whatsmeow message event into the local Message
// model and persists it. For previously-unseen group chats, schedules a
// one-shot group-info fetch to back-fill the display name.
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
	if err := c.upsertMessage(ctx, m); err != nil {
		c.logger.Warnf("upsert message %s in chat %s: %v", m.MessageID, m.ChatJID, err)
	}
	if !m.IsGroup && m.SenderName != "" {
		c.updateChatDisplayName(ctx, m.ChatJID, m.SenderName)
	}
	if m.IsGroup {
		// Best-effort: backfill the group's display name once.
		go c.backfillGroupName(m.ChatJID)
	}
}

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
		_ = am
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
		_ = sm
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
