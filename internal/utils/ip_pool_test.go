package utils

import (
	"fmt"
	"math/big"
	"net"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewIpPoolRejectsInvalidEntry(t *testing.T) {
	_, err := NewIpPool([]string{"192.168.2.10/32", "invalid-cidr"})
	require.ErrorContains(t, err, "invalid-cidr")
}

func TestIpPoolReturnsUsableAddresses(t *testing.T) {
	for _, tc := range []struct {
		name     string
		subnets  []string
		expected []string
	}{
		{
			// subnets with 8 or more addresses exclude the first two and the last one
			name:    "IPv4",
			subnets: []string{"192.168.1.0/29", "192.168.2.0/30", "10.0.0.5"},
			expected: []string{
				"192.168.1.2", "192.168.1.3", "192.168.1.4", "192.168.1.5", "192.168.1.6",
				"192.168.2.0", "192.168.2.1", "192.168.2.2", "192.168.2.3",
				"10.0.0.5",
			},
		},
		{
			name:    "IPv6",
			subnets: []string{"2001::/125", "2001:db8::/127", "2001:db8:cafe::a"},
			expected: []string{
				"2001::2", "2001::3", "2001::4", "2001::5", "2001::6",
				"2001:db8::", "2001:db8::1",
				"2001:db8:cafe::a",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool, err := NewIpPool(tc.subnets)
			require.NoError(t, err)
			byKey, random := map[string]bool{}, map[string]bool{}
			for i := 0; i < 1000; i++ {
				byKey[pool.GetByKey(fmt.Sprint(i)).String()] = true
				random[pool.GetRandomly().String()] = true
			}
			expected := make([]string, len(tc.expected))
			for i, ip := range tc.expected {
				expected[i] = net.ParseIP(ip).String()
			}
			sort.Strings(expected)
			require.Equal(t, expected, sortedKeys(byKey))
			require.Equal(t, expected, sortedKeys(random))
		})
	}
}

func TestIpPoolGetByKeyIsStable(t *testing.T) {
	pool, err := NewIpPool([]string{"2001:db8::/64"})
	require.NoError(t, err)
	require.Equal(t, pool.GetByKey("client"), pool.GetByKey("client"))
}

func TestCalculateUsableIPsForLargeSubnet(t *testing.T) {
	_, ipNet, err := net.ParseCIDR("2001:db8::/64")
	require.NoError(t, err)
	expected := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 64), big.NewInt(3))
	require.Equal(t, expected, calculateUsableIPs(ipNet))
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
