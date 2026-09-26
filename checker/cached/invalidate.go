package cached

// Invalidate drops the cached verdict for userID, so the next HasAccess asks
// the wrapped checker again (used by "recheck" in the Mini App).
func (d *Domain) Invalidate(userID int64) {
	d.mu.Lock()
	delete(d.cache, userID)
	d.mu.Unlock()
}
