package whatsapp

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// ListChats returns chats the local store has observed, ordered by last
// activity. Note: chats only appear after at least one message has been
// observed in this process (whatsmeow does not back-fill history).
func (c *Client) ListChats(ctx context.Context, p ChatListParams) ([]Chat, error) {
	db, err := c.requireLog(ctx)
	if err != nil {
		return nil, err
	}
	limit, offset := pageBounds(p.Limit, 20, 500), p.Offset
	if offset < 0 {
		offset = 0
	}

	var conds []string
	var args []any
	switch {
	case p.OnlyGroups && p.OnlyDirect:
		return nil, fmt.Errorf("%w: OnlyGroups and OnlyDirect are mutually exclusive", ErrInvalidParams)
	case p.OnlyGroups:
		conds = append(conds, "is_group = 1")
	case p.OnlyDirect:
		conds = append(conds, "is_group = 0")
	}
	if p.OnlyUnread {
		conds = append(conds, "unread_count > 0")
	}
	if q := strings.TrimSpace(p.NameQuery); q != "" {
		conds = append(conds, "(LOWER(COALESCE(display_name, '')) LIKE ? OR LOWER(jid) LIKE ?)")
		needle := "%" + strings.ToLower(q) + "%"
		args = append(args, needle, needle)
	}

	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	q := fmt.Sprintf(`
SELECT jid, COALESCE(display_name, ''), is_group, COALESCE(last_msg_at, 0), unread_count
FROM chats
%s
ORDER BY last_msg_at DESC NULLS LAST
LIMIT ? OFFSET ?`, where)
	args = append(args, limit, offset)

	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Chat{}
	for rows.Next() {
		var ch Chat
		var isGroup int
		var lastTs int64
		if err := rows.Scan(&ch.JID, &ch.DisplayName, &isGroup, &lastTs, &ch.UnreadCount); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrParseFailed, err)
		}
		ch.IsGroup = isGroup == 1
		if lastTs > 0 {
			ch.LastActivityAt = time.UnixMilli(lastTs)
		}
		out = append(out, ch)
	}
	return out, rows.Err()
}

// GetMessages returns messages for a chat from the local log, newest
// first. Marks the chat as read.
func (c *Client) GetMessages(ctx context.Context, p MessageListParams) ([]Message, error) {
	db, err := c.requireLog(ctx)
	if err != nil {
		return nil, err
	}
	jid := NormalizeJID(p.JID)
	if jid == "" {
		return nil, fmt.Errorf("%w: JID is required", ErrInvalidJID)
	}
	limit := pageBounds(p.Limit, 50, 500)

	conds := []string{"chat_jid = ?"}
	args := []any{jid}
	if p.BeforeID > 0 {
		conds = append(conds, "rowid < ?")
		args = append(args, p.BeforeID)
	}
	if !p.Since.IsZero() {
		conds = append(conds, "timestamp >= ?")
		args = append(args, p.Since.UnixMilli())
	}
	if !p.Until.IsZero() {
		conds = append(conds, "timestamp <= ?")
		args = append(args, p.Until.UnixMilli())
	}
	q := fmt.Sprintf(`
SELECT rowid, message_id, chat_jid, sender_jid, is_from_me, is_group,
       COALESCE(body, ''), COALESCE(media_kind, ''), COALESCE(media_filename, ''),
       COALESCE(quoted_id, ''), COALESCE(pushname, ''), has_media, timestamp
FROM messages
WHERE %s
ORDER BY timestamp DESC, rowid DESC
LIMIT ?`, strings.Join(conds, " AND "))
	args = append(args, limit)

	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out, err := scanMessages(rows)
	if err != nil {
		return nil, err
	}
	c.markChatRead(ctx, jid)
	return out, nil
}

// requireLog returns the local SQLite handle, returning ErrStoreInit if
// the client has not been Connect()/PairQR()'d yet.
func (c *Client) requireLog(ctx context.Context) (*sql.DB, error) {
	c.mu.RLock()
	db := c.logDB
	closed := c.closed
	c.mu.RUnlock()
	if closed {
		return nil, ErrClosed
	}
	if db != nil {
		return db, nil
	}
	if err := c.initLogStore(ctx); err != nil {
		return nil, err
	}
	c.mu.RLock()
	db = c.logDB
	c.mu.RUnlock()
	if db == nil {
		return nil, ErrStoreInit
	}
	return db, nil
}

func scanMessages(rows *sql.Rows) ([]Message, error) {
	out := []Message{}
	for rows.Next() {
		var m Message
		var isFromMe, isGroup, hasMedia int
		var ts int64
		var kind string
		if err := rows.Scan(
			&m.ID, &m.MessageID, &m.ChatJID, &m.SenderJID,
			&isFromMe, &isGroup, &m.Body, &kind, &m.MediaName,
			&m.QuotedID, &m.SenderName, &hasMedia, &ts,
		); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrParseFailed, err)
		}
		m.IsFromMe = isFromMe == 1
		m.IsGroup = isGroup == 1
		m.HasMedia = hasMedia == 1
		m.MediaKind = MediaKind(kind)
		if ts > 0 {
			m.Timestamp = time.UnixMilli(ts)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func pageBounds(want, def, max int) int {
	if want <= 0 {
		return def
	}
	if want > max {
		return max
	}
	return want
}
