package whatsapp

import (
	"context"
	"sync"
	"time"

	"go.mau.fi/whatsmeow"
)

// groupNameCache tracks which group JIDs we've already attempted to
// resolve, so we don't hammer the WhatsApp servers per incoming message.
type groupNameCache struct {
	mu   sync.Mutex
	seen map[string]struct{}
}

func (g *groupNameCache) markSeen(jid string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.seen == nil {
		g.seen = map[string]struct{}{}
	}
	if _, ok := g.seen[jid]; ok {
		return false
	}
	g.seen[jid] = struct{}{}
	return true
}

var globalGroupCache = &groupNameCache{}

// backfillGroupName resolves a group's display name once per process and
// persists it into the local chats table. Best-effort; errors are
// swallowed (we'll retry on the next process restart).
func (c *Client) backfillGroupName(jid string) {
	if !IsGroupJID(jid) {
		return
	}
	if !globalGroupCache.markSeen(jid) {
		return
	}
	parsed, err := ParseJID(jid)
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = c.withClient(func(wm *whatsmeow.Client) error {
		info, err := wm.GetGroupInfo(ctx, parsed)
		if err != nil {
			c.logger.Debugf("backfill group %s: %v", jid, err)
			return nil
		}
		if info.Name != "" {
			c.updateChatDisplayName(ctx, jid, info.Name)
		}
		return nil
	})
}
