// Package whatsapp provides a Go client + MCP tool surface for WhatsApp
// on top of go.mau.fi/whatsmeow (the WhatsApp multi-device companion
// protocol).
//
// Authentication uses QR pairing on first connect; the session persists
// in a local SQLite store. After pairing, [Client.Connect] starts a
// long-lived background event consumer that writes incoming messages
// into a local SQLite table so the read-style methods have history to
// query (whatsmeow itself does not persist message history).
//
// All write-style methods (SendMessage, SendMedia, React) honour two
// cross-cutting safety knobs:
//   - [WithRequireConfirm] (default true) requires an explicit Confirm
//     flag from the caller.
//   - [WithAllowedRecipients] denies sends to JIDs outside an allowlist.
package whatsapp

import (
	"database/sql"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"

	"go.mau.fi/whatsmeow"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// Config configures a Client. The zero value uses sensible defaults for
// the current OS.
type Config struct {
	// StoreDir holds whatsmeow's session SQLite (session.db) and our
	// local message log (messages.db). Defaults to
	// $XDG_DATA_HOME/teslashibe/whatsapp-go (or
	// ~/Library/Application Support/teslashibe/whatsapp-go on macOS).
	StoreDir string

	// DeviceName is presented in the phone's Linked Devices list.
	// Defaults to "whatsapp-go".
	DeviceName string
}

// Client is a WhatsApp client. Methods are safe for concurrent use after
// Connect returns. Call Close to release resources.
type Client struct {
	storeDir       string
	deviceName     string
	confirmSends   bool
	allowedJIDs    map[string]struct{}
	maxMediaBytes  int64
	logger         waLog.Logger

	mu              sync.RWMutex
	wmeowClient     *whatsmeow.Client
	wmeowEpoch      uint64 // bumped each time wmeowClient is replaced
	handlerInstalled bool   // true once handleEvent is registered on wmeowClient
	logDB           *sql.DB
	connected       bool
	closed          bool
}

const (
	defaultMaxMediaBytes = 25 * 1024 * 1024 // 25MB cap for base64 round-trips
	defaultDeviceName    = "whatsapp-go"
)

// New constructs a Client. Connect must be called before any read or
// write tool.
func New(cfg Config, opts ...Option) *Client {
	storeDir := cfg.StoreDir
	if storeDir == "" {
		storeDir = defaultStoreDir()
	}
	device := cfg.DeviceName
	if device == "" {
		device = defaultDeviceName
	}
	c := &Client{
		storeDir:      storeDir,
		deviceName:    device,
		confirmSends:  true,
		maxMediaBytes: defaultMaxMediaBytes,
		logger:        waLog.Stdout("whatsapp-go", "WARN", true),
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// Option configures a Client.
type Option func(*Client)

// WithRequireConfirm controls whether send-style methods require an
// explicit confirm flag from the caller. Default: true (safer).
func WithRequireConfirm(require bool) Option {
	return func(c *Client) { c.confirmSends = require }
}

// WithAllowedRecipients restricts send-style methods to a fixed set of
// JIDs. Each entry is normalised before insertion. Pass nil/empty to
// disable the allowlist.
func WithAllowedRecipients(jids []string) Option {
	return func(c *Client) {
		if len(jids) == 0 {
			c.allowedJIDs = nil
			return
		}
		m := make(map[string]struct{}, len(jids))
		for _, j := range jids {
			n := NormalizeJID(j)
			if n == "" {
				continue
			}
			m[n] = struct{}{}
		}
		if len(m) == 0 {
			c.allowedJIDs = nil
			return
		}
		c.allowedJIDs = m
	}
}

// WithMaxMediaBytes caps the size of media returned by GetMedia as
// base64. Default: 25 MiB. Pass 0 for no cap.
func WithMaxMediaBytes(n int64) Option {
	return func(c *Client) { c.maxMediaBytes = n }
}

// WithLogger replaces the whatsmeow logger. Default: WARN-level stdout.
func WithLogger(l waLog.Logger) Option {
	return func(c *Client) {
		if l != nil {
			c.logger = l
		}
	}
}

// Close disconnects (if connected) and releases SQLite handles. Safe to
// call multiple times.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	var firstErr error
	if c.wmeowClient != nil {
		c.wmeowClient.Disconnect()
		c.wmeowClient = nil
		c.handlerInstalled = false
		c.wmeowEpoch++
		c.connected = false
	}
	if c.logDB != nil {
		if err := c.logDB.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		c.logDB = nil
	}
	return firstErr
}

// requireConnected returns an error when the client is not connected.
// Read methods may permit a fallback path against the message log only;
// write methods always require a live connection.
func (c *Client) requireConnected() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed {
		return ErrClosed
	}
	if !c.connected || c.wmeowClient == nil {
		return ErrNotConnected
	}
	return nil
}

// withClient runs fn with the active whatsmeow client under a read lock,
// so callers don't need to hold the mutex across the API call.
func (c *Client) withClient(fn func(wm *whatsmeow.Client) error) error {
	c.mu.RLock()
	wm := c.wmeowClient
	closed := c.closed
	c.mu.RUnlock()
	if closed {
		return ErrClosed
	}
	if wm == nil {
		return ErrNotConnected
	}
	return fn(wm)
}

// recipientAllowed enforces WithAllowedRecipients.
func (c *Client) recipientAllowed(jid string) error {
	if len(c.allowedJIDs) == 0 {
		return nil
	}
	if _, ok := c.allowedJIDs[NormalizeJID(jid)]; ok {
		return nil
	}
	return ErrRecipientNotAllowed
}

func defaultStoreDir() string {
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		return filepath.Join(v, "teslashibe", "whatsapp-go")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".whatsapp-go")
	}
	// macOS / fallback: use Application Support.
	return filepath.Join(home, "Library", "Application Support", "teslashibe", "whatsapp-go")
}

