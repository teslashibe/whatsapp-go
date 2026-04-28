package mcp_test

import (
	"reflect"
	"testing"

	whatsapp "github.com/teslashibe/whatsapp-go"
	wamcp "github.com/teslashibe/whatsapp-go/mcp"
	"github.com/teslashibe/mcptool"
)

func TestEveryClientMethodIsWrappedOrExcluded(t *testing.T) {
	rep := mcptool.Coverage(
		reflect.TypeOf(&whatsapp.Client{}),
		wamcp.Provider{}.Tools(),
		wamcp.Excluded,
	)
	if len(rep.Missing) > 0 {
		t.Fatalf("methods missing MCP exposure (add a tool or list in excluded.go): %v", rep.Missing)
	}
	if len(rep.UnknownExclusions) > 0 {
		t.Fatalf("excluded.go references methods that don't exist on *Client (rename?): %v", rep.UnknownExclusions)
	}
	if len(rep.Wrapped)+len(rep.Excluded) == 0 {
		t.Fatal("no wrapped or excluded methods detected — coverage helper is mis-configured")
	}
}

func TestToolsValidate(t *testing.T) {
	if err := mcptool.ValidateTools(wamcp.Provider{}.Tools()); err != nil {
		t.Fatal(err)
	}
}

func TestPlatformName(t *testing.T) {
	if got := (wamcp.Provider{}).Platform(); got != "whatsapp" {
		t.Errorf("Platform() = %q, want whatsapp", got)
	}
}

func TestToolsHaveWhatsAppPrefix(t *testing.T) {
	const prefix = "whatsapp_"
	for _, tool := range (wamcp.Provider{}).Tools() {
		if len(tool.Name) < len(prefix) || tool.Name[:len(prefix)] != prefix {
			t.Errorf("tool %q lacks %s prefix", tool.Name, prefix)
		}
	}
}
