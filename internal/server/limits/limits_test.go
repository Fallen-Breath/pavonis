package limits

import (
	"testing"

	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/utils"
	"github.com/stretchr/testify/require"
)

func newTestManager(t *testing.T, capacity int, global config.ResourceQuotaConfig, groups ...config.ResourceGroupConfig) *Manager {
	t.Helper()
	manager, err := NewManager(&config.ResourceLimitConfig{
		IdleClientCacheSize: &capacity,
		Global:              global,
		Groups:              groups,
	})
	require.NoError(t, err)
	t.Cleanup(manager.Shutdown)
	return manager
}

func acquireLease(t *testing.T, policy *Policy, ip string) *Lease {
	t.Helper()
	lease, rejection := policy.AcquireLease(ip)
	require.Nil(t, rejection, "expected admission for %s", ip)
	return lease
}

func requireRejection(t *testing.T, policy *Policy, ip string) *Rejection {
	t.Helper()
	_, rejection := policy.AcquireLease(ip)
	require.NotNil(t, rejection, "expected rejection for %s", ip)
	return rejection
}

func TestGlobalAndGroupConcurrency(t *testing.T) {
	global := config.ResourceQuotaConfig{AllClients: config.ClientQuotaConfig{RequestConcurrency: utils.ToPtr(2)}}
	group := config.ResourceGroupConfig{
		Id: "downloads", ResourceQuotaConfig: config.ResourceQuotaConfig{
			AllClients: config.ClientQuotaConfig{RequestConcurrency: utils.ToPtr(1)},
		},
	}
	manager := newTestManager(t, 1, global, group)
	downloads, ungrouped := manager.CreateSitePolicy("test", "downloads"), manager.CreateSitePolicy("test", "")

	download := acquireLease(t, downloads, "192.0.2.1")
	requireRejection(t, downloads, "192.0.2.2")
	other := acquireLease(t, ungrouped, "192.0.2.2")
	requireRejection(t, ungrouped, "192.0.2.3")
	download.Release()
	acquireLease(t, downloads, "192.0.2.3").Release()
	other.Release()
}

func TestRejectedRequestPreservesGlobalRateBudget(t *testing.T) {
	global := config.ResourceQuotaConfig{AllClients: config.ClientQuotaConfig{RequestPerHour: utils.ToPtr(2.0)}}
	group := config.ResourceGroupConfig{
		Id: "downloads", ResourceQuotaConfig: config.ResourceQuotaConfig{
			PerClient: config.ClientQuotaConfig{RequestConcurrency: utils.ToPtr(1)},
		},
	}
	policy := newTestManager(t, 1, global, group).CreateSitePolicy("test", "downloads")

	first := acquireLease(t, policy, "192.0.2.1")
	require.Equal(t, &Rejection{"downloads", "per_client", "request_concurrency"}, requireRejection(t, policy, "192.0.2.1"))
	first.Release()
	acquireLease(t, policy, "192.0.2.2").Release()
	require.Equal(t, &Rejection{"", "all_clients", "request_per_hour"}, requireRejection(t, policy, "192.0.2.3"))
}

func TestIdleCapacityEvictsLeastRecentlyUsedAcrossGroups(t *testing.T) {
	quota := config.ResourceQuotaConfig{PerClient: config.ClientQuotaConfig{RequestPerHour: utils.ToPtr(2.0)}}
	manager := newTestManager(t, 2, config.ResourceQuotaConfig{},
		config.ResourceGroupConfig{Id: "a", ResourceQuotaConfig: quota},
		config.ResourceGroupConfig{Id: "b", ResourceQuotaConfig: quota},
	)
	a, b := manager.CreateSitePolicy("test", "a"), manager.CreateSitePolicy("test", "b")

	acquireLease(t, a, "192.0.2.1").Release()
	acquireLease(t, b, "192.0.2.1").Release()
	acquireLease(t, b, "192.0.2.1").Release()
	acquireLease(t, a, "192.0.2.1").Release() // A is now newer than B, both have exhausted their rate budget
	acquireLease(t, a, "192.0.2.2").Release()

	requireRejection(t, a, "192.0.2.1")
	acquireLease(t, b, "192.0.2.1").Release()
}

func TestIdleEvictionPreservesActiveConcurrency(t *testing.T) {
	global := config.ResourceQuotaConfig{PerClient: config.ClientQuotaConfig{
		RequestConcurrency: utils.ToPtr(1),
		RequestPerHour:     utils.ToPtr(2.0),
	}}
	policy := newTestManager(t, 1, global).CreateSitePolicy("test", "")

	active := acquireLease(t, policy, "192.0.2.1")
	acquireLease(t, policy, "192.0.2.2").Release()
	acquireLease(t, policy, "192.0.2.3").Release()
	requireRejection(t, policy, "192.0.2.1")
	active.Release()
	acquireLease(t, policy, "192.0.2.1").Release()
}
