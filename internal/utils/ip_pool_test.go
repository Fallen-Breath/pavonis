package utils

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"math/big"
	"net"
	"testing"
)

func TestNewIpPool(t *testing.T) {
	tests := []struct {
		name          string
		subnets       []string
		expectedPool  *IpPool // We'll check specific fields, not the whole struct due to rnd
		expectedTotal *big.Int
		expectedNumSN int // Expected number of subnets after filtering
		wantErr       bool
		errContains   string
	}{
		{
			name:          "empty subnets",
			subnets:       []string{},
			expectedTotal: big.NewInt(0),
			expectedNumSN: 0,
			wantErr:       false,
			errContains:   "no valid subnets with usable IPs provided",
		},
		{
			name:        "invalid CIDR",
			subnets:     []string{"invalid"},
			wantErr:     true,
			errContains: "invalid IP or CIDR: invalid",
		},
		{
			name:          "single IPv4 /32",
			subnets:       []string{"192.168.1.1"},
			expectedTotal: big.NewInt(1),
			expectedNumSN: 1,
			wantErr:       false,
		},
		{
			name:          "single IPv4 /24",
			subnets:       []string{"192.168.1.0/24"},
			expectedTotal: big.NewInt(253), // 256 - 3
			expectedNumSN: 1,
			wantErr:       false,
		},
		{
			name:          "single IPv4 /30",
			subnets:       []string{"192.168.1.0/30"},
			expectedTotal: big.NewInt(4), // Uses all 4
			expectedNumSN: 1,
			wantErr:       false,
		},
		{
			name:          "single IPv6 /128",
			subnets:       []string{"2001:db8::1"},
			expectedTotal: big.NewInt(1),
			expectedNumSN: 1,
			wantErr:       false,
		},
		{
			name:          "single IPv6 /64",
			subnets:       []string{"2001:db8:cafe::/64"},
			expectedTotal: new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128-64), big.NewInt(3)),
			expectedNumSN: 1,
			wantErr:       false,
		},
		{
			name:          "single IPv6 /126",
			subnets:       []string{"2001:db8::/126"},
			expectedTotal: big.NewInt(4), // Uses all 4
			expectedNumSN: 1,
			wantErr:       false,
		},
		{
			name: "multiple subnets",
			subnets: []string{
				"192.168.1.1",      // 1 IP
				"10.0.0.0/30",      // 4 IPs
				"2001:db8:1::/127", // 2 IPs
			},
			expectedTotal: big.NewInt(1 + 4 + 2),
			expectedNumSN: 3,
			wantErr:       false,
		},
		{
			name: "subnet with zero usable IPs (e.g. hypothetical future rule)",
			// This test case relies on current calculateUsableIPs behavior.
			// If calculateUsableIPs were to return 0 for some valid CIDRs, this test would be more relevant.
			// For now, all valid CIDRs produce >0 usable IPs.
			// Let's test a case that gets filtered out by `numIPs.Cmp(big.NewInt(0)) <= 0` (if that was possible)
			// This is hard to test directly unless we modify calculateUsableIPs or provide a CIDR that ParseCIDR allows but is effectively empty.
			// The current code ensures ParseCIDR itself must succeed and then checks usable IPs.
			// The "no valid subnets" case covers if ALL subnets are filtered.
			// Consider "192.168.1.0/32" which is valid, network address 192.168.1.0, 1 usable IP.
			subnets:       []string{"192.168.1.0/32"},
			expectedTotal: big.NewInt(1),
			expectedNumSN: 1,
			wantErr:       false,
		},
		{
			name: "mixed valid and invalid",
			subnets: []string{
				"192.168.2.10/32",
				"invalid-cidr",
				"10.10.0.0/24",
			},
			wantErr:     true,
			errContains: "invalid IP or CIDR: invalid-cidr",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool, err := NewIpPool(tt.subnets)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expectedTotal, pool.total)
			assert.Len(t, pool.subnets, tt.expectedNumSN)
			assert.Len(t, pool.weights, tt.expectedNumSN)
			if tt.expectedNumSN > 0 {
				assert.NotNil(t, pool.rnd)
			}
		})
	}
}

func Test_ipFromIndex_And_ipFromSubnet(t *testing.T) {
	// This test is more of an integration test for the core IP generation logic.
	// We construct a pool and then try to get every possible IP by iterating index.
	testCases := []struct {
		name        string
		subnets     []string
		expectedIPs []string // In order of generation by index
	}{
		{
			name:    "IPv4 specific cases",
			subnets: []string{"192.168.1.0/29", "192.168.2.0/30", "172.16.0.0/31", "10.0.0.5/32"},
			expectedIPs: []string{
				"192.168.1.2", "192.168.1.3", "192.168.1.4", "192.168.1.5", "192.168.1.6",
				"192.168.2.0", "192.168.2.1", "192.168.2.2", "192.168.2.3",
				"172.16.0.0", "172.16.0.1",
				"10.0.0.5",
			},
		},
		{
			name:    "IPv6 specific cases",
			subnets: []string{"2001::/125", "2001:db8::/126", "2001:db8:feed::/127", "2001:db8:cafe::a/128"},
			expectedIPs: []string{
				"2001::2", "2001::3", "2001::4", "2001::5", "2001::6",
				"2001:db8::", "2001:db8::1", "2001:db8::2", "2001:db8::3",
				"2001:db8:feed::", "2001:db8:feed::1",
				"2001:db8:cafe::a",
			},
		},
		{
			name:        "Single IP Subnet /32",
			subnets:     []string{"1.1.1.1/32"},
			expectedIPs: []string{"1.1.1.1"},
		},
		{
			name:        "Single IP String",
			subnets:     []string{"2.2.2.2"}, // Treated as /32
			expectedIPs: []string{"2.2.2.2"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pool, err := NewIpPool(tc.subnets)
			require.NoError(t, err)
			assert.Equal(t, big.NewInt(int64(len(tc.expectedIPs))), pool.total)
			var generatedIPs []string
			for i := int64(0); i < pool.total.Int64(); i++ {
				ip := pool.ipFromIndex(big.NewInt(i))
				require.NotNil(t, ip)
				generatedIPs = append(generatedIPs, ip.String())
			}
			canonicalExpectedIPs := make([]string, len(tc.expectedIPs))
			for i, ipStr := range tc.expectedIPs {
				canonicalExpectedIPs[i] = net.ParseIP(ipStr).String()
			}
			assert.Equal(t, canonicalExpectedIPs, generatedIPs)
		})
	}
}

func TestCalculateUsableIPs(t *testing.T) {
	tests := []struct {
		name     string
		cidr     string
		expected *big.Int
	}{
		{"ipv4 /32", "192.168.0.1/32", big.NewInt(1)},
		{"ipv4 /31", "192.168.0.0/31", big.NewInt(2)},
		{"ipv4 /30", "192.168.0.0/30", big.NewInt(4)},
		{"ipv4 /30", "192.168.0.0/29", big.NewInt(8 - 3)},
		{"ipv4 /24", "192.168.0.0/24", big.NewInt(256 - 3)},
		{"ipv4 /16", "10.0.0.0/16", big.NewInt(65536 - 3)},
		{"ipv4 /8", "10.0.0.0/8", new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 24), big.NewInt(3))},

		{"ipv6 /128", "2001:db8::1/128", big.NewInt(1)},
		{"ipv6 /127", "2001:db8::/127", big.NewInt(2)},
		{"ipv6 /126", "2001:db8::/126", big.NewInt(4)},
		{"ipv6 /126", "2001:db8::/125", big.NewInt(8 - 3)},
		{"ipv6 /120", "2001:db8::/120", big.NewInt(256 - 3)},
		{"ipv6 /64", "2001:db8:cafe::/64", new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 64), big.NewInt(3))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ipNet, err := net.ParseCIDR(tt.cidr)
			assert.NoError(t, err, "Failed to parse CIDR %s", tt.cidr)

			got := calculateUsableIPs(ipNet)
			assert.Equal(t, tt.expected, got, "Usable IPs for %s mismatch", tt.cidr)
		})
	}
}
