package whatsapp

import (
	"fmt"
	"strings"

	"go.mau.fi/whatsmeow/types"
)

const (
	defaultUserServer  = "s.whatsapp.net"
	defaultGroupServer = "g.us"
)

// NormalizeJID returns a canonical JID string for equality comparison.
//
//   - "+1 (415) 555-1212"          -> "14155551212@s.whatsapp.net"
//   - "14155551212"                -> "14155551212@s.whatsapp.net"
//   - "14155551212@s.whatsapp.net" -> "14155551212@s.whatsapp.net"
//   - "abc-def@g.us"               -> "abc-def@g.us"
//
// Returns "" on inputs we cannot canonicalise (callers should treat that
// as ErrInvalidJID).
func NormalizeJID(in string) string {
	in = strings.TrimSpace(in)
	if in == "" {
		return ""
	}
	if i := strings.IndexByte(in, '@'); i >= 0 {
		user := strings.TrimSpace(in[:i])
		server := strings.TrimSpace(in[i+1:])
		if server == "" {
			server = defaultUserServer
		}
		if server == defaultUserServer {
			user = stripPhone(user)
		} else {
			user = strings.ToLower(user)
		}
		if user == "" {
			return ""
		}
		return user + "@" + server
	}
	// No '@': treat as phone number.
	digits := stripPhone(in)
	if digits == "" {
		return ""
	}
	return digits + "@" + defaultUserServer
}

// ParseJID converts a user-supplied JID string into a whatsmeow types.JID.
// Returns ErrInvalidJID when the input cannot be normalised.
func ParseJID(in string) (types.JID, error) {
	norm := NormalizeJID(in)
	if norm == "" {
		return types.JID{}, fmt.Errorf("%w: %q", ErrInvalidJID, in)
	}
	jid, err := types.ParseJID(norm)
	if err != nil {
		return types.JID{}, fmt.Errorf("%w: %v", ErrInvalidJID, err)
	}
	return jid, nil
}

// IsGroupJID reports whether the canonical JID targets a group chat.
func IsGroupJID(s string) bool {
	return strings.HasSuffix(NormalizeJID(s), "@"+defaultGroupServer)
}

func stripPhone(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	out := b.String()
	if out == "" {
		return ""
	}
	// US 10-digit -> prepend country code 1.
	if len(out) == 10 {
		return "1" + out
	}
	return out
}
