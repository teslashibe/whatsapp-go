package whatsapp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

// SendMessage sends a plain-text message to a JID (user or group).
func (c *Client) SendMessage(ctx context.Context, p SendParams) error {
	body := strings.TrimSpace(p.Body)
	if body == "" {
		return ErrMessageEmpty
	}
	jid, err := ParseJID(p.JID)
	if err != nil {
		return err
	}
	if c.confirmSends && !p.Confirm {
		return ErrConfirmRequired
	}
	if err := c.recipientAllowed(jid.String()); err != nil {
		return err
	}

	var msg *waE2E.Message
	if strings.TrimSpace(p.QuotedID) == "" {
		msg = &waE2E.Message{Conversation: proto.String(p.Body)}
	} else {
		msg = &waE2E.Message{
			ExtendedTextMessage: &waE2E.ExtendedTextMessage{
				Text: proto.String(p.Body),
				ContextInfo: &waE2E.ContextInfo{
					StanzaID: proto.String(p.QuotedID),
				},
			},
		}
	}

	return c.withClient(func(wm *whatsmeow.Client) error {
		resp, err := wm.SendMessage(ctx, jid, msg)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrSendFailed, err)
		}
		// Mirror the outgoing send into the local log so subsequent
		// GetMessages reflects it without waiting for the echo event.
		_ = c.upsertMessage(ctx, Message{
			MessageID:  resp.ID,
			ChatJID:    jid.String(),
			SenderJID:  ownJID(wm),
			IsFromMe:   true,
			IsGroup:    IsGroupJID(jid.String()),
			Body:       p.Body,
			QuotedID:   p.QuotedID,
			Timestamp:  time.Now(),
		})
		return nil
	})
}

// React posts a tapback emoji against a target message. Pass an empty
// Emoji to remove a reaction.
func (c *Client) React(ctx context.Context, p ReactParams) error {
	jid, err := ParseJID(p.JID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(p.MessageID) == "" {
		return fmt.Errorf("%w: MessageID required", ErrInvalidParams)
	}
	if c.confirmSends && !p.Confirm {
		return ErrConfirmRequired
	}
	if err := c.recipientAllowed(jid.String()); err != nil {
		return err
	}
	emoji := p.Emoji // empty = remove
	msg := &waE2E.Message{
		ReactionMessage: &waE2E.ReactionMessage{
			Key: &waCommon.MessageKey{
				RemoteJID: proto.String(jid.String()),
				FromMe:    proto.Bool(p.FromMe),
				ID:        proto.String(p.MessageID),
			},
			Text:              proto.String(emoji),
			SenderTimestampMS: proto.Int64(time.Now().UnixMilli()),
		},
	}
	return c.withClient(func(wm *whatsmeow.Client) error {
		_, err := wm.SendMessage(ctx, jid, msg)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrSendFailed, err)
		}
		return nil
	})
}

// ownJID returns the JID for the locally-paired account, or "" if not
// available. Used for log enrichment of outgoing messages.
func ownJID(wm *whatsmeow.Client) string {
	if wm == nil || wm.Store == nil || wm.Store.ID == nil {
		return ""
	}
	return wm.Store.ID.String()
}
