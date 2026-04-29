package mcp

import (
	"context"
	"time"

	whatsapp "github.com/teslashibe/whatsapp-go"
	"github.com/teslashibe/mcptool"
)

type ListChatsInput struct {
	Limit      int    `json:"limit,omitempty" jsonschema:"description=number of chats to return,minimum=1,maximum=500,default=20"`
	Offset     int    `json:"offset,omitempty" jsonschema:"description=pagination offset,minimum=0,default=0"`
	OnlyGroups bool   `json:"only_groups,omitempty" jsonschema:"description=only return group chats,default=false"`
	OnlyDirect bool   `json:"only_direct,omitempty" jsonschema:"description=only return 1:1 chats,default=false"`
	OnlyUnread bool   `json:"only_unread,omitempty" jsonschema:"description=only return chats with unread messages,default=false"`
	NameQuery  string `json:"name_query,omitempty" jsonschema:"description=optional substring filter against chat display name or JID"`
}

func listChats(ctx context.Context, c *whatsapp.Client, in ListChatsInput) (any, error) {
	res, err := c.ListChats(ctx, whatsapp.ChatListParams{
		Limit:      in.Limit,
		Offset:     in.Offset,
		OnlyGroups: in.OnlyGroups,
		OnlyDirect: in.OnlyDirect,
		OnlyUnread: in.OnlyUnread,
		NameQuery:  in.NameQuery,
	})
	if err != nil {
		return nil, err
	}
	limit := in.Limit
	if limit <= 0 {
		limit = 20
	}
	return mcptool.PageOf(res, "", limit), nil
}

type GetMessagesInput struct {
	JID       string `json:"jid" jsonschema:"description=chat JID (phone or group ID),required"`
	Limit     int    `json:"limit,omitempty" jsonschema:"description=messages per page,minimum=1,maximum=500,default=50"`
	BeforeID  int64  `json:"before_id,omitempty" jsonschema:"description=pagination cursor: only return messages with rowid < before_id"`
	SinceUnix int64  `json:"since_unix,omitempty" jsonschema:"description=lower bound on send time, unix seconds"`
	UntilUnix int64  `json:"until_unix,omitempty" jsonschema:"description=upper bound on send time, unix seconds"`
}

func getMessages(ctx context.Context, c *whatsapp.Client, in GetMessagesInput) (any, error) {
	p := whatsapp.MessageListParams{JID: in.JID, Limit: in.Limit, BeforeID: in.BeforeID}
	if in.SinceUnix > 0 {
		p.Since = time.Unix(in.SinceUnix, 0)
	}
	if in.UntilUnix > 0 {
		p.Until = time.Unix(in.UntilUnix, 0)
	}
	res, err := c.GetMessages(ctx, p)
	if err != nil {
		return nil, err
	}
	limit := in.Limit
	if limit <= 0 {
		limit = 50
	}
	return mcptool.PageOf(res, "", limit), nil
}

var chatTools = []mcptool.Tool{
	mcptool.Define[*whatsapp.Client, ListChatsInput](
		"whatsapp_list_chats",
		"List WhatsApp chats from the local message log, ordered by last activity",
		"ListChats",
		listChats,
	),
	mcptool.Define[*whatsapp.Client, GetMessagesInput](
		"whatsapp_get_messages",
		"Fetch messages for a chat from the local log, paginated and optionally bounded by time",
		"GetMessages",
		getMessages,
	),
}
