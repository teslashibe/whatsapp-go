package whatsapp

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

// ResolveContact searches whatsmeow's local contact store for entries
// matching query (substring against any of pushname/firstname/fullname/
// businessname; exact match against the normalised JID).
func (c *Client) ResolveContact(ctx context.Context, query string) ([]Contact, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, fmt.Errorf("%w: query is required", ErrInvalidParams)
	}
	if err := c.requireConnected(); err != nil {
		return nil, err
	}
	var out []Contact
	err := c.withClient(func(wm *whatsmeow.Client) error {
		all, err := wm.Store.Contacts.GetAllContacts(ctx)
		if err != nil {
			return err
		}
		needle := strings.ToLower(q)
		jidNorm := NormalizeJID(query)
		for jid, info := range all {
			if matchesContact(info, jid, needle, jidNorm) {
				out = append(out, Contact{
					JID:       jid.String(),
					PushName:  info.PushName,
					FirstName: info.FirstName,
					FullName:  info.FullName,
					Business:  info.BusinessName,
				})
			}
		}
		sort.Slice(out, func(i, j int) bool {
			return contactDisplay(out[i]) < contactDisplay(out[j])
		})
		return nil
	})
	return out, err
}

func contactDisplay(c Contact) string {
	if c.FullName != "" {
		return c.FullName
	}
	if c.PushName != "" {
		return c.PushName
	}
	return c.JID
}

func matchesContact(info types.ContactInfo, jid types.JID, needle, jidNorm string) bool {
	if needle != "" {
		for _, s := range []string{info.PushName, info.FirstName, info.FullName, info.BusinessName} {
			if s != "" && strings.Contains(strings.ToLower(s), needle) {
				return true
			}
		}
	}
	if jidNorm != "" && jid.String() == jidNorm {
		return true
	}
	return false
}

// IsOnWhatsApp checks one or more phone numbers against WhatsApp's
// presence service. Numbers should be in E.164 form (or any form that
// NormalizeJID can canonicalise).
func (c *Client) IsOnWhatsApp(ctx context.Context, phones []string) (map[string]bool, error) {
	if len(phones) == 0 {
		return nil, fmt.Errorf("%w: phones is required", ErrInvalidParams)
	}
	if err := c.requireConnected(); err != nil {
		return nil, err
	}
	// Normalise to "+digits" form (whatsmeow expects the leading + and digits).
	canonical := make([]string, 0, len(phones))
	rev := make(map[string]string, len(phones)) // canonical -> original
	out := map[string]bool{}                    // pre-populated with false for unresolvable
	for _, p := range phones {
		jid := NormalizeJID(p)
		// Reject group JIDs and anything we couldn't normalise.
		if jid == "" || IsGroupJID(jid) {
			out[p] = false
			continue
		}
		user := jid[:strings.IndexByte(jid, '@')]
		key := "+" + user
		canonical = append(canonical, key)
		rev[key] = p
	}
	if len(canonical) == 0 {
		return out, nil
	}
	err := c.withClient(func(wm *whatsmeow.Client) error {
		results, err := wm.IsOnWhatsApp(ctx, canonical)
		if err != nil {
			return err
		}
		for _, r := range results {
			orig := rev[r.Query]
			if orig == "" {
				orig = r.Query
			}
			out[orig] = r.IsIn
		}
		return nil
	})
	return out, err
}
