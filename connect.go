package whatsapp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
)

// Connect brings the client online. If the device is not yet paired,
// returns ErrPairingRequired; the caller should then invoke PairQR,
// scan the returned code with the WhatsApp app, and call Connect again.
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return ErrClosed
	}
	if c.connected && c.wmeowClient != nil {
		c.mu.Unlock()
		return ErrAlreadyConnected
	}
	c.mu.Unlock()

	if err := c.initLogStore(ctx); err != nil {
		return err
	}

	wm, paired, err := c.openWhatsmeow(ctx)
	if err != nil {
		return err
	}
	if !paired {
		// Don't connect a fresh device implicitly — that opens a QR
		// channel without a consumer. Caller must invoke PairQR.
		return ErrPairingRequired
	}

	// Wire event handler before Connect so we don't race the first event.
	wm.AddEventHandler(c.handleEvent)
	if err := wm.Connect(); err != nil {
		return fmt.Errorf("whatsapp: connect: %w", err)
	}

	c.mu.Lock()
	c.wmeowClient = wm
	c.connected = true
	c.mu.Unlock()
	return nil
}

// Disconnect tears down the websocket without forgetting the pairing.
// Subsequent Connect calls reuse the same device.
func (c *Client) Disconnect(ctx context.Context) error {
	c.mu.Lock()
	wm := c.wmeowClient
	c.wmeowClient = nil
	c.connected = false
	c.mu.Unlock()
	if wm != nil {
		wm.Disconnect()
	}
	return nil
}

// PairQR yields a QR code that the user scans from
// Settings → Linked Devices in the WhatsApp app. Blocks until pairing
// succeeds, the context cancels, or the QR sequence times out.
//
// On success, the device session is persisted; subsequent calls to
// Connect skip pairing.
func (c *Client) PairQR(ctx context.Context) (QRPairing, error) {
	if err := c.initLogStore(ctx); err != nil {
		return QRPairing{}, err
	}
	wm, paired, err := c.openWhatsmeow(ctx)
	if err != nil {
		return QRPairing{}, err
	}
	if paired {
		// Already paired — connect transparently and return an empty
		// QRPairing so callers can proceed.
		wm.AddEventHandler(c.handleEvent)
		if err := wm.Connect(); err != nil {
			return QRPairing{}, fmt.Errorf("whatsapp: connect: %w", err)
		}
		c.mu.Lock()
		c.wmeowClient = wm
		c.connected = true
		c.mu.Unlock()
		return QRPairing{}, nil
	}

	qrChan, err := wm.GetQRChannel(ctx)
	if err != nil {
		return QRPairing{}, fmt.Errorf("whatsapp: open QR channel: %w", err)
	}
	wm.AddEventHandler(c.handleEvent)
	if err := wm.Connect(); err != nil {
		return QRPairing{}, fmt.Errorf("whatsapp: connect during pair: %w", err)
	}

	var firstCode QRPairing
	for evt := range qrChan {
		switch evt.Event {
		case whatsmeow.QRChannelEventCode:
			ascii := renderQRASCII(evt.Code)
			pairing := QRPairing{
				Code:      evt.Code,
				ASCII:     ascii,
				ExpiresAt: time.Now().Add(evt.Timeout),
			}
			if firstCode.Code == "" {
				firstCode = pairing
			}
			// Continue: next iteration is either a fresh code or success.
		case "success":
			c.mu.Lock()
			c.wmeowClient = wm
			c.connected = true
			c.mu.Unlock()
			return firstCode, nil
		case "timeout":
			wm.Disconnect()
			return firstCode, ErrPairingTimeout
		default:
			wm.Disconnect()
			return firstCode, fmt.Errorf("%w: %s", ErrPairingCancelled, evt.Event)
		}
	}
	wm.Disconnect()
	if ctx.Err() != nil {
		return firstCode, ctx.Err()
	}
	return firstCode, ErrPairingCancelled
}

// openWhatsmeow constructs (or loads) a whatsmeow.Client from the on-disk
// session store. Returns paired=false when no session exists yet.
func (c *Client) openWhatsmeow(ctx context.Context) (*whatsmeow.Client, bool, error) {
	dsn := "file:" + filepath.Join(c.storeDir, "session.db") +
		"?_pragma=foreign_keys(on)&_pragma=busy_timeout(2000)"
	container, err := sqlstore.New(ctx, "sqlite", dsn, c.logger)
	if err != nil {
		return nil, false, fmt.Errorf("whatsapp: open session store: %w", err)
	}
	device, err := container.GetFirstDevice(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("whatsapp: get device: %w", err)
	}
	wm := whatsmeow.NewClient(device, c.logger)
	if c.deviceName != "" {
		// whatsmeow exposes its device props at the package level via
		// store.DeviceProps; we leave this default to avoid cross-process
		// surprises. The DeviceName is only echoed in StatusReport.
		_ = c.deviceName
	}
	paired := device.ID != nil
	return wm, paired, nil
}

// renderQRASCII writes the QR for code as a half-block ASCII string
// suitable for terminal display.
func renderQRASCII(code string) string {
	var buf bytes.Buffer
	cfg := qrterminal.Config{
		Level:     qrterminal.M,
		Writer:    &buf,
		BlackChar: qrterminal.BLACK,
		WhiteChar: qrterminal.WHITE,
		QuietZone: 1,
	}
	qrterminal.GenerateWithConfig(code, cfg)
	return buf.String()
}

// errClosing helper to make linter happy on unused imports during early dev.
var _ = errors.Is
