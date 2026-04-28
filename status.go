package whatsapp

import (
	"context"
)

// Status reports whether the client is paired, connected, and logged in,
// plus the local store metadata. Designed as the first tool an agent
// calls when troubleshooting.
func (c *Client) Status(ctx context.Context) (StatusReport, error) {
	rep := StatusReport{
		DeviceName: c.deviceName,
		StoreDir:   c.storeDir,
		HelpPair:   "Call PairQR to display a QR code, then scan it from the WhatsApp app under Settings → Linked Devices → Link a device.",
	}
	if err := c.initLogStore(ctx); err == nil {
		rep.MessageLogReady = true
		rep.StoredMessageCount = c.storedMessageCount(ctx)
	}

	c.mu.RLock()
	wm := c.wmeowClient
	c.mu.RUnlock()
	if wm == nil {
		return rep, nil
	}
	rep.Connected = wm.IsConnected()
	rep.LoggedIn = wm.IsLoggedIn()
	if wm.Store != nil && wm.Store.ID != nil {
		rep.Paired = true
		rep.OwnJID = wm.Store.ID.String()
	}
	return rep, nil
}
