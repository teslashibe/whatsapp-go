package whatsapp

import "testing"

func TestWithAllowedRecipients_DropsEmpty(t *testing.T) {
	c := New(Config{}, WithAllowedRecipients([]string{"", "  ", "+1 (415) 555-1212"}))
	if _, ok := c.allowedJIDs[""]; ok {
		t.Fatalf("empty JID leaked into allowlist: %v", c.allowedJIDs)
	}
	if len(c.allowedJIDs) != 1 {
		t.Fatalf("expected exactly 1 entry, got %d (%v)", len(c.allowedJIDs), c.allowedJIDs)
	}
	if err := c.recipientAllowed(""); err == nil {
		t.Fatalf("empty recipient should not be allowed")
	}
	if err := c.recipientAllowed("14155551212@s.whatsapp.net"); err != nil {
		t.Fatalf("normalised match should be allowed; got %v", err)
	}
}

func TestWithAllowedRecipients_NilDisablesGuard(t *testing.T) {
	c := New(Config{}, WithAllowedRecipients(nil))
	if c.allowedJIDs != nil {
		t.Fatalf("nil input should disable allowlist; got %v", c.allowedJIDs)
	}
	if err := c.recipientAllowed("anything@s.whatsapp.net"); err != nil {
		t.Fatalf("disabled allowlist should permit any recipient; got %v", err)
	}
}

func TestWithAllowedRecipients_AllEmptyDisablesGuard(t *testing.T) {
	c := New(Config{}, WithAllowedRecipients([]string{"", "  "}))
	if c.allowedJIDs != nil {
		t.Fatalf("entirely-empty allowlist should be treated as disabled; got %v", c.allowedJIDs)
	}
}
