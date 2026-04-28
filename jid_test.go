package whatsapp

import "testing"

func TestNormalizeJID(t *testing.T) {
	cases := map[string]string{
		"+1 (415) 555-1212":           "14155551212@s.whatsapp.net",
		"4155551212":                  "14155551212@s.whatsapp.net",
		"14155551212@s.whatsapp.net":  "14155551212@s.whatsapp.net",
		"14155551212@":                "14155551212@s.whatsapp.net",
		"abc-123-def@g.us":            "abc-123-def@g.us",
		" 14155551212 ":               "14155551212@s.whatsapp.net",
		"":                            "",
		"@@":                          "",
	}
	for in, want := range cases {
		if got := NormalizeJID(in); got != want {
			t.Errorf("NormalizeJID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsGroupJID(t *testing.T) {
	if !IsGroupJID("abc@g.us") {
		t.Error("expected g.us suffix to be a group")
	}
	if IsGroupJID("14155551212@s.whatsapp.net") {
		t.Error("expected user JID to not be a group")
	}
}

func TestParseJIDError(t *testing.T) {
	if _, err := ParseJID(""); err == nil {
		t.Error("expected error on empty input")
	}
	if _, err := ParseJID("@@"); err == nil {
		t.Error("expected error on garbage input")
	}
}
