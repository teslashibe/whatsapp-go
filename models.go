package whatsapp

import "time"

// MediaKind enumerates supported attachment kinds.
type MediaKind string

const (
	MediaImage    MediaKind = "image"
	MediaVideo    MediaKind = "video"
	MediaAudio    MediaKind = "audio"
	MediaDocument MediaKind = "document"
	MediaSticker  MediaKind = "sticker"
)

// Chat is a conversation thread (one-on-one or group).
type Chat struct {
	JID            string    `json:"jid"`
	DisplayName    string    `json:"displayName,omitempty"`
	IsGroup        bool      `json:"isGroup"`
	LastActivityAt time.Time `json:"lastActivityAt,omitempty"`
	UnreadCount    int       `json:"unreadCount,omitempty"`
}

// Message is a single message stored in the local log.
type Message struct {
	ID           int64     `json:"id"`
	MessageID    string    `json:"messageId"`
	ChatJID      string    `json:"chatJid"`
	SenderJID    string    `json:"senderJid,omitempty"`
	SenderName   string    `json:"senderName,omitempty"`
	IsFromMe     bool      `json:"isFromMe"`
	IsGroup      bool      `json:"isGroup"`
	Body         string    `json:"body,omitempty"`
	MediaKind    MediaKind `json:"mediaKind,omitempty"`
	MediaName    string    `json:"mediaName,omitempty"`
	QuotedID     string    `json:"quotedMessageId,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
	HasMedia     bool      `json:"hasMedia,omitempty"`
}

// Contact is a resolved address-book entry from whatsmeow's contact store.
type Contact struct {
	JID       string `json:"jid"`
	PushName  string `json:"pushName,omitempty"`
	FirstName string `json:"firstName,omitempty"`
	FullName  string `json:"fullName,omitempty"`
	Business  string `json:"businessName,omitempty"`
}

// MediaPayload is returned by GetMedia.
type MediaPayload struct {
	MessageID string    `json:"messageId"`
	Kind      MediaKind `json:"kind"`
	MIMEType  string    `json:"mimeType,omitempty"`
	Filename  string    `json:"filename,omitempty"`
	Size      int64     `json:"size"`
	DataB64   string    `json:"dataBase64"`
}

// QRPairing carries the QR codes the user scans on their phone.
type QRPairing struct {
	// Code is the raw QR payload string. Use ASCII or Image to display.
	Code string `json:"code"`
	// ASCII renders the QR as a terminal-friendly ASCII art block.
	ASCII string `json:"ascii"`
	// ExpiresAt is when this code rotates; whatsmeow emits a new one
	// every ~30 seconds until success or timeout.
	ExpiresAt time.Time `json:"expiresAt"`
}

// StatusReport summarises the current client state.
type StatusReport struct {
	Paired               bool   `json:"paired"`
	Connected            bool   `json:"connected"`
	LoggedIn             bool   `json:"loggedIn"`
	OwnJID               string `json:"ownJid,omitempty"`
	DeviceName           string `json:"deviceName"`
	StoreDir             string `json:"storeDir"`
	MessageLogReady      bool   `json:"messageLogReady"`
	StoredMessageCount   int64  `json:"storedMessageCount"`
	HelpPair             string `json:"helpPair,omitempty"`
}

// --- Param structs ---

// ChatListParams configures ListChats.
type ChatListParams struct {
	Limit       int    // default 20, max 500
	Offset      int
	OnlyGroups  bool
	OnlyDirect  bool
	OnlyUnread  bool
	NameQuery   string // optional: substring filter against display name
}

// MessageListParams configures GetMessages.
type MessageListParams struct {
	JID      string // required
	Limit    int    // default 50, max 500
	BeforeID int64  // pagination: only messages with rowid < BeforeID
	Since    time.Time
	Until    time.Time
}

// SearchParams configures Search.
type SearchParams struct {
	Query    string
	JID      string    // optional chat filter
	Sender   string    // optional sender filter
	Since    time.Time
	Until    time.Time
	Limit    int
	Offset   int
	OnlyMine *bool
}

// SendParams configures SendMessage.
type SendParams struct {
	JID     string // required (recipient or group JID)
	Body    string // required
	QuotedID string // optional: reply to this message ID
	Confirm bool
}

// SendMediaParams configures SendMedia.
type SendMediaParams struct {
	JID      string
	FilePath string
	Kind     MediaKind // image/video/audio/document/sticker; auto-detected if empty
	Caption  string
	MIMEType string // optional override
	Confirm  bool
}

// ReactParams configures React.
type ReactParams struct {
	JID         string // chat JID
	MessageID   string // target message ID
	Emoji       string // single emoji (empty = remove reaction)
	FromMe      bool   // whether the target message is one we sent (affects key)
	Confirm     bool
}

// WatchParams configures Watch.
type WatchParams struct {
	SinceID int64
	JID     string
	Limit   int
}

// WatchResult is the return shape for Watch.
type WatchResult struct {
	Messages []Message `json:"messages"`
	Cursor   int64     `json:"cursor"`
}
