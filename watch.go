package whatsapp

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Watch returns messages with rowid > p.SinceID, plus a new cursor.
// Bootstrap with SinceID=0 to fetch the current MAX(rowid) without
// pulling any messages.
func (c *Client) Watch(ctx context.Context, p WatchParams) (WatchResult, error) {
	db, err := c.requireLog(ctx)
	if err != nil {
		return WatchResult{}, err
	}
	limit := pageBounds(p.Limit, 100, 500)

	if p.SinceID <= 0 {
		var max sql.NullInt64
		if err := db.QueryRowContext(ctx, `SELECT MAX(rowid) FROM messages`).Scan(&max); err != nil {
			return WatchResult{}, err
		}
		return WatchResult{Messages: []Message{}, Cursor: max.Int64}, nil
	}

	conds := []string{"rowid > ?"}
	args := []any{p.SinceID}
	if jid := NormalizeJID(p.JID); jid != "" {
		conds = append(conds, "chat_jid = ?")
		args = append(args, jid)
	}
	q := fmt.Sprintf(`
SELECT rowid, message_id, chat_jid, sender_jid, is_from_me, is_group,
       COALESCE(body, ''), COALESCE(media_kind, ''), COALESCE(media_filename, ''),
       COALESCE(quoted_id, ''), COALESCE(pushname, ''), has_media, timestamp
FROM messages
WHERE %s
ORDER BY rowid ASC
LIMIT ?`, strings.Join(conds, " AND "))
	args = append(args, limit)

	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return WatchResult{}, err
	}
	defer rows.Close()
	msgs, err := scanMessages(rows)
	if err != nil {
		return WatchResult{}, err
	}
	cursor := p.SinceID
	for _, m := range msgs {
		if m.ID > cursor {
			cursor = m.ID
		}
	}
	return WatchResult{Messages: msgs, Cursor: cursor}, nil
}
