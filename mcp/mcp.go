// Package mcp exposes the whatsapp-go [whatsapp.Client] surface as a set
// of MCP (Model Context Protocol) tools that any host application can
// mount on its own MCP server.
//
// All tools wrap exported methods on *whatsapp.Client. Each tool is
// defined via [mcptool.Define] so the JSON input schema is reflected
// from the typed input struct — no hand-maintained schemas, no drift.
//
// Usage from a host application:
//
//	import (
//	    "github.com/teslashibe/mcptool"
//	    whatsapp "github.com/teslashibe/whatsapp-go"
//	    wamcp "github.com/teslashibe/whatsapp-go/mcp"
//	)
//
//	client := whatsapp.New(whatsapp.Config{}, whatsapp.WithRequireConfirm(true))
//	for _, tool := range wamcp.Provider{}.Tools() {
//	    // register tool with your MCP server, passing client as the client arg
//	}
//
// The [Excluded] map documents methods on *Client that are intentionally
// not exposed via MCP. The coverage test in mcp_test.go fails if a new
// exported method is added without either being wrapped by a tool or
// appearing in [Excluded].
package mcp

import "github.com/teslashibe/mcptool"

// Provider implements [mcptool.Provider] for whatsapp-go.
type Provider struct{}

// Platform returns "whatsapp".
func (Provider) Platform() string { return "whatsapp" }

// Tools returns every whatsapp-go MCP tool, in registration order.
func (Provider) Tools() []mcptool.Tool {
	out := make([]mcptool.Tool, 0,
		len(statusTools)+len(authTools)+len(chatTools)+
			len(searchTools)+len(sendTools)+len(reactTools)+
			len(contactTools)+len(watchTools))
	out = append(out, statusTools...)
	out = append(out, authTools...)
	out = append(out, chatTools...)
	out = append(out, searchTools...)
	out = append(out, sendTools...)
	out = append(out, reactTools...)
	out = append(out, contactTools...)
	out = append(out, watchTools...)
	return out
}
