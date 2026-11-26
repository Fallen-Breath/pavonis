package resourcelimit

import (
	"time"

	"github.com/hashicorp/golang-lru/v2/simplelru"
)

type clientKey struct {
	scope    *scope
	ipBucket string
}

type clientQuota struct {
	quota
	key clientKey
}

type idleClient struct {
	quota   *clientQuota
	expires time.Time
}

// clientTable keeps the per-client quotas. A quota in use is never evicted, so concurrent requests
// of a client always count on the same quota. Idle quotas wait in a bounded LRU until they refill
type clientTable struct {
	active map[clientKey]*clientQuota
	idle   *simplelru.LRU[clientKey, idleClient]
}

func newClientTable(idleCapacity int) (*clientTable, error) {
	idle, err := simplelru.NewLRU[clientKey, idleClient](idleCapacity, nil)
	if err != nil {
		return nil, err
	}
	return &clientTable{active: make(map[clientKey]*clientQuota), idle: idle}, nil
}

// getOrCreateClientQuota does not change the table, so a rejected request leaves no trace
func (t *clientTable) getOrCreateClientQuota(key clientKey) *clientQuota {
	if c := t.active[key]; c != nil {
		return c
	}
	if entry, ok := t.idle.Peek(key); ok {
		return entry.quota
	}
	return &clientQuota{quota: *newQuota(key.scope.perClientConfig), key: key}
}

func (t *clientTable) activateClientQuota(c *clientQuota) {
	t.idle.Remove(c.key)
	t.active[c.key] = c
}

func (t *clientTable) retireClientQuota(c *clientQuota, now time.Time) {
	delete(t.active, c.key)
	if c.key.scope.idleTTL > 0 {
		t.idle.Add(c.key, idleClient{quota: c, expires: now.Add(c.key.scope.idleTTL)})
	}
}

func (t *clientTable) removeExpiredIdleQuotas(now time.Time) {
	// Scopes have different TTLs, so expiry order differs from LRU order
	for _, key := range t.idle.Keys() {
		if entry, _ := t.idle.Peek(key); !entry.expires.After(now) {
			t.idle.Remove(key)
		}
	}
}
