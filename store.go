package whatsapp

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// initLogStore opens (or creates) the local message-log SQLite at
// <storeDir>/messages.db and runs the schema migrations.
func (c *Client) initLogStore(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.logDB != nil {
		return nil
	}
	if err := os.MkdirAll(c.storeDir, 0o700); err != nil {
		return fmt.Errorf("%w: mkdir %s: %v", ErrStoreInit, c.storeDir, err)
	}
	dsn := "file:" + filepath.Join(c.storeDir, "messages.db") +
		"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(2000)&_pragma=foreign_keys(on)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("%w: open: %v", ErrStoreInit, err)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return fmt.Errorf("%w: ping: %v", ErrStoreInit, err)
	}
	if err := migrate(ctx, db); err != nil {
		db.Close()
		return fmt.Errorf("%w: migrate: %v", ErrStoreInit, err)
	}
	c.logDB = db
	return nil
}

func migrate(ctx context.Context, db *sql.DB) error {
	const schema = `
CREATE TABLE IF NOT EXISTS messages (
    rowid          INTEGER PRIMARY KEY AUTOINCREMENT,
    message_id     TEXT NOT NULL,
    chat_jid       TEXT NOT NULL,
    sender_jid     TEXT NOT NULL,
    is_from_me     INTEGER NOT NULL,
    is_group       INTEGER NOT NULL,
    body           TEXT,
    media_kind     TEXT,
    media_filename TEXT,
    quoted_id      TEXT,
    pushname       TEXT,
    has_media      INTEGER NOT NULL DEFAULT 0,
    timestamp      INTEGER NOT NULL,
    created_at     INTEGER NOT NULL,
    UNIQUE(message_id, chat_jid)
);
CREATE INDEX IF NOT EXISTS idx_msg_chat_ts  ON messages(chat_jid, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_msg_body     ON messages(body);
CREATE INDEX IF NOT EXISTS idx_msg_sender   ON messages(sender_jid);

CREATE TABLE IF NOT EXISTS chats (
    jid             TEXT PRIMARY KEY,
    display_name    TEXT,
    is_group        INTEGER NOT NULL,
    last_msg_at     INTEGER,
    unread_count    INTEGER NOT NULL DEFAULT 0
);`
	_, err := db.ExecContext(ctx, schema)
	return err
}

// upsertMessage inserts an incoming or outgoing message into the log,
// idempotent on (message_id, chat_jid). Also touches the chats row.
func (c *Client) upsertMessage(ctx context.Context, m Message) error {
	c.mu.RLock()
	db := c.logDB
	c.mu.RUnlock()
	if db == nil {
		return nil // log not initialised; tolerate
	}
	const q = `
INSERT OR REPLACE INTO messages
  (message_id, chat_jid, sender_jid, is_from_me, is_group, body, media_kind,
   media_filename, quoted_id, pushname, has_media, timestamp, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	hasMedia := 0
	if m.HasMedia {
		hasMedia = 1
	}
	if _, err := db.ExecContext(ctx, q,
		m.MessageID, m.ChatJID, m.SenderJID,
		boolToInt(m.IsFromMe), boolToInt(m.IsGroup),
		nullable(m.Body), nullable(string(m.MediaKind)),
		nullable(m.MediaName), nullable(m.QuotedID),
		nullable(m.SenderName), hasMedia,
		m.Timestamp.UnixMilli(), time.Now().UnixMilli(),
	); err != nil {
		return err
	}
	const upsertChat = `
INSERT INTO chats (jid, display_name, is_group, last_msg_at, unread_count)
VALUES (?, '', ?, ?, ?)
ON CONFLICT(jid) DO UPDATE SET
  last_msg_at = MAX(COALESCE(last_msg_at, 0), excluded.last_msg_at),
  unread_count = unread_count + excluded.unread_count`
	unread := 0
	if !m.IsFromMe {
		unread = 1
	}
	_, err := db.ExecContext(ctx, upsertChat,
		m.ChatJID, boolToInt(m.IsGroup), m.Timestamp.UnixMilli(), unread,
	)
	return err
}

// markChatRead zeroes the unread counter for a chat. Called by GetMessages
// to mirror "user opened the chat" semantics.
func (c *Client) markChatRead(ctx context.Context, jid string) {
	c.mu.RLock()
	db := c.logDB
	c.mu.RUnlock()
	if db == nil {
		return
	}
	_, _ = db.ExecContext(ctx, `UPDATE chats SET unread_count = 0 WHERE jid = ?`, jid)
}

// updateChatDisplayName sets/refreshes the display name for a chat.
func (c *Client) updateChatDisplayName(ctx context.Context, jid, name string) {
	if strings.TrimSpace(name) == "" {
		return
	}
	c.mu.RLock()
	db := c.logDB
	c.mu.RUnlock()
	if db == nil {
		return
	}
	_, _ = db.ExecContext(ctx, `
INSERT INTO chats (jid, display_name, is_group, last_msg_at, unread_count)
VALUES (?, ?, ?, 0, 0)
ON CONFLICT(jid) DO UPDATE SET display_name = excluded.display_name
WHERE COALESCE(display_name, '') = '' OR display_name <> excluded.display_name`,
		jid, name, boolToInt(IsGroupJID(jid)),
	)
}

// storedMessageCount is used by Status.
func (c *Client) storedMessageCount(ctx context.Context) int64 {
	c.mu.RLock()
	db := c.logDB
	c.mu.RUnlock()
	if db == nil {
		return 0
	}
	var n int64
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM messages`).Scan(&n); err != nil {
		return 0
	}
	return n
}

// errIsNoRows abstracts the SQLite "no rows" case.
func errIsNoRows(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
