package whatsapp

import (
	"context"
	"fmt"
	"strings"
)

// Search performs case-insensitive substring search across stored
// message bodies, with optional chat/sender/time/direction filters.
func (c *Client) Search(ctx context.Context, p SearchParams) ([]Message, error) {
	if strings.TrimSpace(p.Query) == "" {
		return nil, fmt.Errorf("%w: query is required", ErrInvalidParams)
	}
	db, err := c.requireLog(ctx)
	if err != nil {
		return nil, err
	}
	limit := pageBounds(p.Limit, 50, 500)
	offset := p.Offset
	if offset < 0 {
		offset = 0
	}

	conds := []string{"LOWER(COALESCE(body, '')) LIKE ?"}
	args := []any{"%" + strings.ToLower(p.Query) + "%"}
	if jid := NormalizeJID(p.JID); jid != "" {
		conds = append(conds, "chat_jid = ?")
		args = append(args, jid)
	}
	if s := NormalizeJID(p.Sender); s != "" {
		conds = append(conds, "sender_jid = ?")
		args = append(args, s)
	}
	if !p.Since.IsZero() {
		conds = append(conds, "timestamp >= ?")
		args = append(args, p.Since.UnixMilli())
	}
	if !p.Until.IsZero() {
		conds = append(conds, "timestamp <= ?")
		args = append(args, p.Until.UnixMilli())
	}
	if p.OnlyMine != nil {
		if *p.OnlyMine {
			conds = append(conds, "is_from_me = 1")
		} else {
			conds = append(conds, "is_from_me = 0")
		}
	}

	q := fmt.Sprintf(`
SELECT rowid, message_id, chat_jid, sender_jid, is_from_me, is_group,
       COALESCE(body, ''), COALESCE(media_kind, ''), COALESCE(media_filename, ''),
       COALESCE(quoted_id, ''), COALESCE(pushname, ''), has_media, timestamp
FROM messages
WHERE %s
ORDER BY timestamp DESC
LIMIT ? OFFSET ?`, strings.Join(conds, " AND "))
	args = append(args, limit, offset)

	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMessages(rows)
}
