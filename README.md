# whatsapp-go

A Go client + MCP tool surface for WhatsApp on top of
[`go.mau.fi/whatsmeow`](https://github.com/tulir/whatsmeow) (the WhatsApp
multi-device companion protocol). Drop-in compatible with
[`teslashibe/mcptool`](https://github.com/teslashibe/mcptool) hosts
(Cursor, Claude Desktop, Claude Code).

```go
import "github.com/teslashibe/whatsapp-go"
```

## How it works

WhatsApp doesn't expose a local message database the way iMessage does
(no `chat.db`). This package therefore:

1. Pairs to your WhatsApp account once via QR (the same flow as
   WhatsApp Web), persisting the session in a local SQLite store.
2. Maintains a long-lived connection that subscribes to WhatsApp's
   event stream.
3. Writes incoming messages and receipts into a small local SQLite
   `messages` table (separate from whatsmeow's session store) so the
   read-style MCP tools have history to query.

```
┌──────────────────┐  QR pair (once)   ┌────────────┐
│ whatsapp-go      │ ◀────────────────▶│ WhatsApp   │
│   ├─ whatsmeow   │   websocket       │ multidevice│
│   ├─ session.db  │   event stream    │ servers    │
│   └─ messages.db │ ◀─────────────────│            │
└──────────────────┘                   └────────────┘
        ▲
        │ MCP tools (read messages.db; write via whatsmeow)
        │
   Cursor/Claude
```

## Install

```bash
go get github.com/teslashibe/whatsapp-go
```

## Quick start

```go
client := whatsapp.New(whatsapp.Config{
    StoreDir: "/Users/me/.config/teslashibe/whatsapp-go",
},
    whatsapp.WithRequireConfirm(true),
    whatsapp.WithAllowedRecipients([]string{"+14155551212"}),
)
defer client.Close()

if err := client.Connect(ctx); err != nil {
    if errors.Is(err, whatsapp.ErrPairingRequired) {
        // Print the QR; scan it from Settings → Linked Devices on phone
        qr, _ := client.PairQR(ctx)
        fmt.Println(qr.ASCII)
        client.Connect(ctx) // re-call after scan
    }
}

// Read
chats, _ := client.ListChats(ctx, whatsapp.ChatListParams{Limit: 10})
msgs, _ := client.GetMessages(ctx, whatsapp.MessageListParams{
    JID: "14155551212@s.whatsapp.net", Limit: 20,
})

// Write
_ = client.SendMessage(ctx, whatsapp.SendParams{
    JID:     "14155551212@s.whatsapp.net",
    Body:    "ack",
    Confirm: true,
})
```

## Capability surface

### V1 (MVP)
| Method | Tool |
|---|---|
| `Status` | `whatsapp_status` |
| `PairQR` | `whatsapp_pair_qr` |
| `Connect` / `Disconnect` | `whatsapp_connect`, `whatsapp_disconnect` |
| `ListChats` | `whatsapp_list_chats` |
| `GetMessages` | `whatsapp_get_messages` |
| `Search` | `whatsapp_search` |
| `SendMessage` | `whatsapp_send_message` |
| `ResolveContact` | `whatsapp_resolve_contact` |

### V1.0.2
| Method | Tool |
|---|---|
| `SendMedia` | `whatsapp_send_media` |
| `React` | `whatsapp_react` (proper reactions; whatsmeow supports them natively) |
| `IsOnWhatsApp` | `whatsapp_check_whatsapp` |

### V2 stretch
| Method | Tool |
|---|---|
| `Watch` | `whatsapp_watch` |

### Not yet implemented (deliberately)
- `GetMedia` (download attachments by message ID). Persisting raw event
  protos is required to reconstruct the encrypted download URL and PII
  policy needs separate review; tracked as a follow-up.

## Permissions & operational notes

- **Pairing.** First connect requires scanning a QR with the WhatsApp
  app on a paired phone. The session persists in `<StoreDir>/session.db`
  and survives restarts.
- **Background connection.** `Connect` starts a long-running goroutine
  that consumes the event stream until `Disconnect`/`Close`. The host
  must keep the process alive.
- **Allowlist guard.** `WithAllowedRecipients` denies sends to off-list
  JIDs (matched after normalising to `<digits>@s.whatsapp.net`).
- **Confirm guard.** Default-on; every send-style tool requires
  `confirm: true` from the agent.

## Drift prevention

`mcp/mcp_test.go` runs `mcptool.Coverage` over `*whatsapp.Client`. Any
new exported method that isn't wrapped by an MCP tool or listed in
`mcp.Excluded` with a one-line reason fails the build.

## License

MIT (this package). The whatsmeow dependency is MPL-2.0.
