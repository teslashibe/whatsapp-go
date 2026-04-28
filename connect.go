package whatsapp

import (
	"bytes"
	"context"
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

	// Install the handler under the lock so a concurrent Disconnect can't
	// orphan us, and only register once per client instance.
	c.installHandler(wm)
	if err := wm.Connect(); err != nil {
		c.mu.Lock()
		if c.wmeowClient == wm {
			c.wmeowClient = nil
			c.handlerInstalled = false
		}
		c.mu.Unlock()
		return fmt.Errorf("whatsapp: connect: %w", err)
	}
	c.mu.Lock()
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
	c.handlerInstalled = false
	c.wmeowEpoch++
	c.connected = false
	c.mu.Unlock()
	if wm != nil {
		wm.Disconnect()
	}
	return nil
}

// PairQR begins QR pairing and returns the first scannable QR code as
// soon as it's available. The returned ASCII rendering is suitable for
// terminal display; the raw Code is also returned for callers that want
// to render it themselves.
//
// If the device is already paired, PairQR transparently connects and
// returns an empty QRPairing. If pairing fails, ErrPairingTimeout or
// ErrPairingCancelled is returned.
//
// NOTE: this call returns once whatsmeow emits the first QR code (~1s),
// not when the user finishes scanning. Successful pairing is signalled
// by Status() reporting Paired=true; agents should poll Status after
// the user scans.
func (c *Client) PairQR(ctx context.Context) (QRPairing, error) {
	if err := c.initLogStore(ctx); err != nil {
		return QRPairing{}, err
	}
	wm, paired, err := c.openWhatsmeow(ctx)
	if err != nil {
		return QRPairing{}, err
	}
	if paired {
		c.installHandler(wm)
		if err := wm.Connect(); err != nil {
			return QRPairing{}, fmt.Errorf("whatsapp: connect: %w", err)
		}
		c.mu.Lock()
		c.connected = true
		c.mu.Unlock()
		return QRPairing{}, nil
	}

	qrChan, err := wm.GetQRChannel(ctx)
	if err != nil {
		return QRPairing{}, fmt.Errorf("whatsapp: open QR channel: %w", err)
	}
	c.installHandler(wm)
	if err := wm.Connect(); err != nil {
		return QRPairing{}, fmt.Errorf("whatsapp: connect during pair: %w", err)
	}

	// Wait for the first code (or terminal event); spin off a goroutine
	// to drain the rest of the channel so whatsmeow's pairing flow
	// completes server-side.
	for evt := range qrChan {
		switch evt.Event {
		case whatsmeow.QRChannelEventCode:
			pairing := QRPairing{
				Code:      evt.Code,
				ASCII:     renderQRASCII(evt.Code),
				ExpiresAt: time.Now().Add(evt.Timeout),
			}
			go drainQRChannel(c, wm, qrChan)
			return pairing, nil
		case "success":
			c.mu.Lock()
			c.connected = true
			c.mu.Unlock()
			return QRPairing{}, nil
		case "timeout":
			c.teardown(wm)
			return QRPairing{}, ErrPairingTimeout
		default:
			c.teardown(wm)
			return QRPairing{}, fmt.Errorf("%w: %s", ErrPairingCancelled, evt.Event)
		}
	}
	c.teardown(wm)
	if ctx.Err() != nil {
		return QRPairing{}, ctx.Err()
	}
	return QRPairing{}, ErrPairingCancelled
}

// drainQRChannel consumes remaining QR rotations until pairing succeeds,
// fails, or times out. On terminal events we update connection state and
// log; we do *not* tear down on success because the active connection
// continues to be used.
func drainQRChannel(c *Client, wm *whatsmeow.Client, ch <-chan whatsmeow.QRChannelItem) {
	for evt := range ch {
		switch evt.Event {
		case "success":
			c.mu.Lock()
			if c.wmeowClient == wm {
				c.connected = true
			}
			c.mu.Unlock()
			return
		case "timeout":
			c.logger.Warnf("QR pairing timed out before scan")
			c.teardown(wm)
			return
		case whatsmeow.QRChannelEventCode:
			// Subsequent rotations: nothing to do; the agent is
			// expected to call Status to poll for Paired=true.
		default:
			c.logger.Warnf("QR channel terminal event: %s", evt.Event)
			c.teardown(wm)
			return
		}
	}
}

// teardown disconnects wm if it is still the active client and clears
// the handler flag. Safe to call from goroutines.
func (c *Client) teardown(wm *whatsmeow.Client) {
	c.mu.Lock()
	active := c.wmeowClient == wm
	if active {
		c.wmeowClient = nil
		c.handlerInstalled = false
		c.wmeowEpoch++
		c.connected = false
	}
	c.mu.Unlock()
	if wm != nil {
		wm.Disconnect()
	}
}

// installHandler atomically attaches the active client and registers
// handleEvent exactly once for that client.
func (c *Client) installHandler(wm *whatsmeow.Client) {
	c.mu.Lock()
	if c.wmeowClient != wm {
		c.wmeowClient = wm
		c.handlerInstalled = false
		c.wmeowEpoch++
	}
	if !c.handlerInstalled {
		epoch := c.wmeowEpoch
		wm.AddEventHandler(c.makeHandler(epoch))
		c.handlerInstalled = true
	}
	c.mu.Unlock()
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
