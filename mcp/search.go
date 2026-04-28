package mcp

import (
	"context"
	"time"

	whatsapp "github.com/teslashibe/whatsapp-go"
	"github.com/teslashibe/mcptool"
)

type SearchInput struct {
	Query     string `json:"query" jsonschema:"description=case-insensitive substring to search for,required"`
	JID       string `json:"jid,omitempty" jsonschema:"description=optional chat JID filter"`
	Sender    string `json:"sender,omitempty" jsonschema:"description=optional sender JID filter"`
	SinceUnix int64  `json:"since_unix,omitempty" jsonschema:"description=lower bound on send time, unix seconds"`
	UntilUnix int64  `json:"until_unix,omitempty" jsonschema:"description=upper bound on send time, unix seconds"`
	Limit     int    `json:"limit,omitempty" jsonschema:"description=results per page,minimum=1,maximum=500,default=50"`
	Offset    int    `json:"offset,omitempty" jsonschema:"description=pagination offset,minimum=0,default=0"`
	OnlyMine  *bool  `json:"only_mine,omitempty" jsonschema:"description=true=only my messages\\, false=only theirs\\, omit=both"`
}

func search(ctx context.Context, c *whatsapp.Client, in SearchInput) (any, error) {
	p := whatsapp.SearchParams{
		Query: in.Query, JID: in.JID, Sender: in.Sender,
		Limit: in.Limit, Offset: in.Offset, OnlyMine: in.OnlyMine,
	}
	if in.SinceUnix > 0 {
		p.Since = time.Unix(in.SinceUnix, 0)
	}
	if in.UntilUnix > 0 {
		p.Until = time.Unix(in.UntilUnix, 0)
	}
	res, err := c.Search(ctx, p)
	if err != nil {
		return nil, err
	}
	limit := in.Limit
	if limit <= 0 {
		limit = 50
	}
	return mcptool.PageOf(res, "", limit), nil
}

var searchTools = []mcptool.Tool{
	mcptool.Define[*whatsapp.Client, SearchInput](
		"whatsapp_search",
		"Substring search across stored WhatsApp message bodies with optional chat/sender/time/direction filters",
		"Search",
		search,
	),
}
