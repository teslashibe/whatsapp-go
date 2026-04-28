package mcp

import (
	"context"

	whatsapp "github.com/teslashibe/whatsapp-go"
	"github.com/teslashibe/mcptool"
)

type ResolveContactInput struct {
	Query string `json:"query" jsonschema:"description=name substring or exact phone/JID to match in the WhatsApp contact store,required"`
}

func resolveContact(ctx context.Context, c *whatsapp.Client, in ResolveContactInput) (any, error) {
	return c.ResolveContact(ctx, in.Query)
}

type IsOnWhatsAppInput struct {
	Phones []string `json:"phones" jsonschema:"description=phone numbers in any form (E.164 preferred); duplicates are ignored,required"`
}

func isOnWhatsApp(ctx context.Context, c *whatsapp.Client, in IsOnWhatsAppInput) (any, error) {
	return c.IsOnWhatsApp(ctx, in.Phones)
}

var contactTools = []mcptool.Tool{
	mcptool.Define[*whatsapp.Client, ResolveContactInput](
		"whatsapp_resolve_contact",
		"Resolve a name or JID to one or more entries from the WhatsApp contact store",
		"ResolveContact",
		resolveContact,
	),
	mcptool.Define[*whatsapp.Client, IsOnWhatsAppInput](
		"whatsapp_check_whatsapp",
		"Check whether one or more phone numbers are registered on WhatsApp",
		"IsOnWhatsApp",
		isOnWhatsApp,
	),
}
