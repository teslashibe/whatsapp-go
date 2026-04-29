package whatsapp

import "errors"

var (
	ErrClosed              = errors.New("whatsapp: client is closed")
	ErrNotConnected        = errors.New("whatsapp: client is not connected; call Connect first")
	ErrAlreadyConnected    = errors.New("whatsapp: client is already connected")
	ErrPairingRequired     = errors.New("whatsapp: device is not paired; call PairQR and scan with the WhatsApp app")
	ErrPairingTimeout      = errors.New("whatsapp: QR pairing timed out without a scan")
	ErrPairingCancelled    = errors.New("whatsapp: QR pairing was cancelled")
	ErrInvalidParams       = errors.New("whatsapp: invalid parameters")
	ErrInvalidJID          = errors.New("whatsapp: invalid JID (expected <digits>@s.whatsapp.net or <id>@g.us)")
	ErrNotFound            = errors.New("whatsapp: not found")
	ErrConfirmRequired     = errors.New("whatsapp: confirm=true required for send-style operations")
	ErrRecipientNotAllowed = errors.New("whatsapp: recipient is not in the configured allowlist")
	ErrMessageEmpty        = errors.New("whatsapp: message body is empty")
	ErrMediaTooLarge       = errors.New("whatsapp: media exceeds configured size cap")
	ErrUnsupportedMedia    = errors.New("whatsapp: message has no downloadable media")
	ErrSendFailed          = errors.New("whatsapp: send failed")
	ErrParseFailed         = errors.New("whatsapp: failed to parse stored row")
	ErrStoreInit           = errors.New("whatsapp: failed to initialise local store")
)
