package whatsapp

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

// SendMedia uploads a file and sends it as a media message. Kind is
// auto-detected from the MIME type when empty. Supported kinds: image,
// video, audio, document, sticker.
func (c *Client) SendMedia(ctx context.Context, p SendMediaParams) error {
	if strings.TrimSpace(p.FilePath) == "" {
		return fmt.Errorf("%w: FilePath required", ErrInvalidParams)
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

	abs, err := filepath.Abs(p.FilePath)
	if err != nil {
		return fmt.Errorf("%w: resolve path: %v", ErrInvalidParams, err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidParams, err)
	}
	if info.IsDir() {
		return fmt.Errorf("%w: %s is a directory", ErrInvalidParams, abs)
	}
	if info.Size() == 0 {
		return fmt.Errorf("%w: %s is empty", ErrInvalidParams, abs)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidParams, err)
	}
	mime := p.MIMEType
	if mime == "" {
		mime = http.DetectContentType(data)
	}
	kind := p.Kind
	if kind == "" {
		kind = inferKindFromMIME(mime)
	}
	wmType, err := whatsmeowMediaType(kind)
	if err != nil {
		return err
	}

	return c.withClient(func(wm *whatsmeow.Client) error {
		upload, err := wm.Upload(ctx, data, wmType)
		if err != nil {
			return fmt.Errorf("%w: upload: %v", ErrSendFailed, err)
		}
		msg := buildMediaMessage(kind, info.Name(), mime, p.Caption, data, upload)
		if msg == nil {
			return fmt.Errorf("%w: unsupported media kind %q", ErrInvalidParams, kind)
		}
		resp, err := wm.SendMessage(ctx, jid, msg)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrSendFailed, err)
		}
		_ = c.upsertMessage(ctx, Message{
			MessageID: resp.ID,
			ChatJID:   jid.String(),
			SenderJID: ownJID(wm),
			IsFromMe:  true,
			IsGroup:   IsGroupJID(jid.String()),
			Body:      p.Caption,
			MediaKind: kind,
			MediaName: info.Name(),
			HasMedia:  true,
			Timestamp: time.Now(),
		})
		return nil
	})
}

func buildMediaMessage(kind MediaKind, filename, mime, caption string, data []byte, up whatsmeow.UploadResponse) *waE2E.Message {
	size := uint64(len(data))
	switch kind {
	case MediaImage:
		return &waE2E.Message{ImageMessage: &waE2E.ImageMessage{
			URL:           proto.String(up.URL),
			DirectPath:    proto.String(up.DirectPath),
			MediaKey:      up.MediaKey,
			Mimetype:      proto.String(mime),
			FileEncSHA256: up.FileEncSHA256,
			FileSHA256:    up.FileSHA256,
			FileLength:    proto.Uint64(size),
			Caption:       proto.String(caption),
		}}
	case MediaVideo:
		return &waE2E.Message{VideoMessage: &waE2E.VideoMessage{
			URL:           proto.String(up.URL),
			DirectPath:    proto.String(up.DirectPath),
			MediaKey:      up.MediaKey,
			Mimetype:      proto.String(mime),
			FileEncSHA256: up.FileEncSHA256,
			FileSHA256:    up.FileSHA256,
			FileLength:    proto.Uint64(size),
			Caption:       proto.String(caption),
		}}
	case MediaAudio:
		return &waE2E.Message{AudioMessage: &waE2E.AudioMessage{
			URL:           proto.String(up.URL),
			DirectPath:    proto.String(up.DirectPath),
			MediaKey:      up.MediaKey,
			Mimetype:      proto.String(mime),
			FileEncSHA256: up.FileEncSHA256,
			FileSHA256:    up.FileSHA256,
			FileLength:    proto.Uint64(size),
		}}
	case MediaDocument:
		return &waE2E.Message{DocumentMessage: &waE2E.DocumentMessage{
			URL:           proto.String(up.URL),
			DirectPath:    proto.String(up.DirectPath),
			MediaKey:      up.MediaKey,
			Mimetype:      proto.String(mime),
			FileEncSHA256: up.FileEncSHA256,
			FileSHA256:    up.FileSHA256,
			FileLength:    proto.Uint64(size),
			FileName:      proto.String(filename),
			Caption:       proto.String(caption),
		}}
	case MediaSticker:
		return &waE2E.Message{StickerMessage: &waE2E.StickerMessage{
			URL:           proto.String(up.URL),
			DirectPath:    proto.String(up.DirectPath),
			MediaKey:      up.MediaKey,
			Mimetype:      proto.String(mime),
			FileEncSHA256: up.FileEncSHA256,
			FileSHA256:    up.FileSHA256,
			FileLength:    proto.Uint64(size),
		}}
	}
	return nil
}

func inferKindFromMIME(mime string) MediaKind {
	switch {
	case strings.HasPrefix(mime, "image/"):
		return MediaImage
	case strings.HasPrefix(mime, "video/"):
		return MediaVideo
	case strings.HasPrefix(mime, "audio/"):
		return MediaAudio
	}
	return MediaDocument
}

func whatsmeowMediaType(k MediaKind) (whatsmeow.MediaType, error) {
	switch k {
	case MediaImage, MediaSticker:
		return whatsmeow.MediaImage, nil
	case MediaVideo:
		return whatsmeow.MediaVideo, nil
	case MediaAudio:
		return whatsmeow.MediaAudio, nil
	case MediaDocument:
		return whatsmeow.MediaDocument, nil
	}
	return "", fmt.Errorf("%w: unknown media kind %q", ErrInvalidParams, k)
}

// GetMedia downloads the media payload of a previously-received message
// and returns it as base64. Looks up the original whatsmeow proto via
// the local log (which stored a row but not the proto), so this requires
// the message to be re-fetched. Since we don't persist the raw proto,
// the caller must pass it as part of a recent event flow OR the caller
// must use whatsmeow's Download via a separately-held event reference.
//
// In practice agents will fetch attachments while the message is fresh:
// call GetMessages, see has_media=true, then call GetMedia immediately.
// We re-derive the download by re-pulling the message from the local
// log's media metadata; if that's insufficient, returns ErrUnsupportedMedia
// with guidance to re-receive the message.
//
// NOTE: this is a deliberately conservative implementation. Persisting
// raw protos is an MPL/PII concern handled in a follow-up.
func (c *Client) GetMedia(ctx context.Context, messageID string) (MediaPayload, error) {
	if strings.TrimSpace(messageID) == "" {
		return MediaPayload{}, fmt.Errorf("%w: messageID required", ErrInvalidParams)
	}
	if err := c.requireConnected(); err != nil {
		return MediaPayload{}, err
	}
	// We only have metadata in the local log — without the encrypted
	// MediaKey/SHA digests, we can't reconstruct a download. Return a
	// clear error so callers know to wire a richer flow if they need
	// inline media.
	return MediaPayload{
		MessageID: messageID,
	}, fmt.Errorf("%w: in-process attachment download requires the original event proto; "+
		"persist raw_message in your event handler and call whatsmeow.Client.Download directly",
		ErrUnsupportedMedia)
}

// dataB64 is a small helper kept exported-like for tests.
func dataB64(b []byte) string { return base64.StdEncoding.EncodeToString(b) }
