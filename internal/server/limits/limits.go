package limits

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/metrics"
	"github.com/Fallen-Breath/pavonis/internal/utils"
)

type Manager struct {
	mu                 sync.Mutex
	clients            *clientTable
	rejectionResponses chan struct{}
	scopes             map[string][]*scope
	stop               context.CancelFunc
}

// Policy applies the shared quotas to one site
type Policy struct {
	manager *Manager
	scopes  []*scope
	site    string
}

func NewManager(cfg *config.ResourceLimitConfig) (*Manager, error) {
	clients, err := newClientTable(*cfg.IdleClientCacheSize)
	if err != nil {
		return nil, err
	}
	m := &Manager{
		clients:            clients,
		scopes:             make(map[string][]*scope),
		rejectionResponses: make(chan struct{}, *cfg.MaxConcurrent429Responses),
	}

	var globalScopes []*scope
	if s := newScope("", &cfg.Global); s != nil {
		globalScopes = append(globalScopes, s)
	}
	m.scopes[""] = globalScopes
	for _, group := range cfg.Groups {
		scopes := slices.Clone(globalScopes)
		if s := newScope(group.Id, &group.ResourceQuotaConfig); s != nil {
			scopes = append(scopes, s)
		}
		m.scopes[group.Id] = scopes
	}

	ctx, stop := context.WithCancel(context.Background())
	m.stop = stop
	go m.expireIdleClients(ctx)
	return m, nil
}

// CreateSitePolicy binds a site's rejection counters while keeping its quotas shared.
func (m *Manager) CreateSitePolicy(site, group string) *Policy {
	p := &Policy{manager: m, scopes: m.scopes[group], site: site}
	for _, s := range p.scopes {
		initRejectionCounters(site, s.group, "all_clients", s.allClientsConfig)
		initRejectionCounters(site, s.group, "per_client", s.perClientConfig)
	}
	return p
}

// Register at zero so the first rejection after a scrape is visible to rate and increase.
func initRejectionCounters(site, group, quota string, cfg *config.ClientQuotaConfig) {
	if cfg == nil {
		return
	}
	if cfg.RequestConcurrency != nil {
		metrics.GetResourceLimitRejectionCounter(site, group, quota, "request_concurrency")
	}
	for _, r := range getRequestRates(cfg) {
		if r.value != nil {
			metrics.GetResourceLimitRejectionCounter(site, group, quota, r.name)
		}
	}
}

func (m *Manager) Shutdown() {
	m.stop()
}

func (m *Manager) expireIdleClients(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			m.mu.Lock()
			m.clients.removeExpiredIdleQuotas(now)
			m.mu.Unlock()
		}
	}
}

// Lease is the quotas held by an admitted request. It must be released when the request ends
type Lease struct {
	manager        *Manager
	allClients     []*quota
	perClient      []*clientQuota
	TrafficLimiter utils.RateLimiter // nil means unlimited
}

type Rejection struct {
	ResourceGroup string // empty for global
	Quota         string
	Limit         string
}

// AcquireLease admits a request only if all applicable quotas admit it, and takes nothing otherwise
func (p *Policy) AcquireLease(clientAddr string) (*Lease, *Rejection) {
	ipBucket := utils.GetBucketForIpString(clientAddr)
	m := p.manager
	m.mu.Lock()
	defer m.mu.Unlock()

	lease := &Lease{manager: m}
	now := time.Now()
	for _, s := range p.scopes {
		if q := s.allClients; q != nil {
			if limit := q.findExceededLimit(now); limit != "" {
				return nil, &Rejection{s.group, "all_clients", limit}
			}
			lease.allClients = append(lease.allClients, q)
		}
	}
	for _, s := range p.scopes {
		if s.perClientConfig != nil {
			c := m.clients.getOrCreateClientQuota(clientKey{scope: s, ipBucket: ipBucket})
			if limit := c.findExceededLimit(now); limit != "" {
				return nil, &Rejection{s.group, "per_client", limit}
			}
			lease.perClient = append(lease.perClient, c)
		}
	}

	var traffic []utils.RateLimiter
	for _, q := range lease.allClients {
		q.startRequest(now)
		if q.traffic != nil {
			traffic = append(traffic, q.traffic)
		}
	}
	for _, c := range lease.perClient {
		if c.active == 0 {
			m.clients.activateClientQuota(c)
		}
		c.startRequest(now)
		if c.traffic != nil {
			traffic = append(traffic, c.traffic)
		}
	}
	if len(traffic) > 0 {
		lease.TrafficLimiter = utils.NewMultiRateLimiter(traffic...)
	}
	return lease, nil
}

func (l *Lease) Release() {
	l.manager.mu.Lock()
	defer l.manager.mu.Unlock()
	now := time.Now()
	for _, q := range l.allClients {
		q.finishRequest()
	}
	for _, c := range l.perClient {
		c.finishRequest()
		if c.active == 0 {
			l.manager.clients.retireClientQuota(c, now)
		}
	}
}
