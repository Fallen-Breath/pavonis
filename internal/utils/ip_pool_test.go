package utils

import (
	"math/big"
	"net"
	"reflect"
	"strings"
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
			name:        "empty subnets",
			subnets:     []string{},
			wantErr:     true,
			errContains: "no valid subnets with usable IPs provided",
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
			expectedTotal: big.NewInt(254), // 256 - 2
			expectedNumSN: 1,
			wantErr:       false,
		},
		{
			name:          "single IPv4 /30",
			subnets:       []string{"192.168.1.0/30"},
			expectedTotal: big.NewInt(2), // 4 - 2
			expectedNumSN: 1,
			wantErr:       false,
		},
		{
			name:          "single IPv4 /31",
			subnets:       []string{"192.168.1.0/31"},
			expectedTotal: big.NewInt(2), // Uses all 2
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
			expectedTotal: new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128-64), big.NewInt(2)),
			expectedNumSN: 1,
			wantErr:       false,
		},
		{
			name:          "single IPv6 /126",
			subnets:       []string{"2001:db8::/126"},
			expectedTotal: big.NewInt(2), // 4 - 2
			expectedNumSN: 1,
			wantErr:       false,
		},
		{
			name:          "single IPv6 /127",
			subnets:       []string{"2001:db8::/127"},
			expectedTotal: big.NewInt(2), // Uses all 2
			expectedNumSN: 1,
			wantErr:       false,
		},
		{
			name: "multiple subnets",
			subnets: []string{
				"192.168.1.1",      // 1 IP
				"10.0.0.0/30",      // 2 IPs
				"2001:db8:1::/127", // 2 IPs
			},
			expectedTotal: big.NewInt(1 + 2 + 2),
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
		{
			name:          "IPv4 mapped IPv6 CIDR",
			subnets:       []string{"::ffff:192.168.1.0/120"}, // effectively a /24 in IPv4 space
			expectedTotal: big.NewInt(254),                    // 2^(128-120) - 2 = 2^8 - 2 = 256 - 2 = 254
			expectedNumSN: 1,
			wantErr:       false,
		},
		{
			name:          "IPv4 mapped IPv6 single IP",
			subnets:       []string{"::ffff:192.168.1.1"}, // effectively /128
			expectedTotal: big.NewInt(1),
			expectedNumSN: 1,
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool, err := NewIpPool(tt.subnets)
			if tt.wantErr {
				if err == nil {
					t.Errorf("NewIpPool() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("NewIpPool() error = %v, want errContains %s", err, tt.errContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewIpPool() unexpected error = %v", err)
			}

			if pool.total.Cmp(tt.expectedTotal) != 0 {
				t.Errorf("NewIpPool() total = %s, want %s", pool.total.String(), tt.expectedTotal.String())
			}
			if len(pool.subnets) != tt.expectedNumSN {
				t.Errorf("NewIpPool() number of subnets = %d, want %d", len(pool.subnets), tt.expectedNumSN)
			}
			if len(pool.weights) != tt.expectedNumSN {
				t.Errorf("NewIpPool() number of weights = %d, want %d", len(pool.weights), tt.expectedNumSN)
			}
			if tt.expectedNumSN > 0 && pool.rnd == nil {
				t.Errorf("NewIpPool() rnd is nil for a valid pool")
			}
		})
	}
}

func TestIpPool_GetByKey(t *testing.T) {
	pool, err := NewIpPool([]string{
		"192.168.0.0/30", // .1, .2 (2 IPs)
		"10.0.0.1/32",    // .1 (1 IP)
		"2001:db8::/126", // ::1, ::2 (2 IPs)
	}) // Total = 2 + 1 + 2 = 5
	if err != nil {
		t.Fatalf("Failed to create IP pool: %v", err)
	}

	tests := []struct {
		name    string
		key     string
		wantIPs []net.IP // Expected IP, or list of possible IPs if multiple keys could map
	}{
		{
			name:    "key1",
			key:     "user1@example.com",
			wantIPs: []net.IP{net.ParseIP("192.168.0.1"), net.ParseIP("192.168.0.2"), net.ParseIP("10.0.0.1"), net.ParseIP("2001:db8::1"), net.ParseIP("2001:db8::2")},
		},
		{
			name:    "key2",
			key:     "another-service-identifier",
			wantIPs: []net.IP{net.ParseIP("192.168.0.1"), net.ParseIP("192.168.0.2"), net.ParseIP("10.0.0.1"), net.ParseIP("2001:db8::1"), net.ParseIP("2001:db8::2")},
		},
		{
			name:    "key1 again (deterministic)",
			key:     "user1@example.com",
			wantIPs: []net.IP{pool.GetByKey("user1@example.com")}, // Must be same as first call
		},
	}

	key1IP := pool.GetByKey("user1@example.com") // Get it once for deterministic check

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotIP := pool.GetByKey(tt.key)
			if gotIP == nil {
				t.Errorf("GetByKey() got nil IP")
				return
			}

			found := false
			for _, wantIP := range tt.wantIPs {
				if gotIP.Equal(wantIP) {
					found = true
					break
				}
			}
			if !found {
				// This check is tricky because hash can map anywhere.
				// The crucial part is that it's within *some* valid range.
				// The provided tt.wantIPs is more of a "possible set".
				// Better check: Is it one of the IPs we expect generally?
				t.Logf("GetByKey() got %s, which was not in the explicit wantIPs list for the test case, but this is often fine due to hashing.", gotIP.String())
			}

			if tt.name == "key1 again (deterministic)" {
				if !gotIP.Equal(key1IP) {
					t.Errorf("GetByKey() for key '%s' is not deterministic: got %s, previously %s", tt.key, gotIP, key1IP)
				}
			}

			// Verify the IP is contained in one of the subnets and is a usable IP
			isValidIP := false
			for _, sn := range pool.subnets {
				if sn.Contains(gotIP) {
					// Check if it's network or broadcast for subnets >= /30 or /126
					ones, bits := sn.Mask.Size()
					if (bits - ones) <= 2 { // For /30, /31, /32 (IPv4) or /126, /127, /128 (IPv6)
						numIPs := big.NewInt(1)
						numIPs.Lsh(numIPs, uint(bits-ones))
						if numIPs.Cmp(big.NewInt(4)) >= 0 { // /30 or /126
							networkIP := sn.IP
							ipInt := big.NewInt(0).SetBytes(networkIP)
							broadcastIPInt := big.NewInt(0).Add(ipInt, numIPs)
							broadcastIPInt.Sub(broadcastIPInt, big.NewInt(1))

							broadcastIPBytes := broadcastIPInt.Bytes()
							paddedBroadcastIPBytes := make([]byte, len(networkIP))
							copy(paddedBroadcastIPBytes[len(paddedBroadcastIPBytes)-len(broadcastIPBytes):], broadcastIPBytes)
							broadcastIP := net.IP(paddedBroadcastIPBytes)

							if gotIP.Equal(networkIP) || gotIP.Equal(broadcastIP) {
								t.Errorf("GetByKey() returned network or broadcast IP %s for subnet %s", gotIP, sn.String())
							}
						}
					}
					isValidIP = true
					break
				}
			}
			if !isValidIP {
				t.Errorf("GetByKey() returned IP %s not in any configured subnet", gotIP)
			}
		})
	}
}

func TestIpPool_GetRandomly(t *testing.T) {
	subnets := []string{
		"172.16.0.0/30", // .1, .2
		"172.16.0.4/31", // .4, .5
		"172.16.0.6/32", // .6
		"fd00::/126",    // ::1, ::2
		"fd00::4/127",   // ::4, ::5
		"fd00::6/128",   // ::6
	}
	pool, err := NewIpPool(subnets)
	if err != nil {
		t.Fatalf("Failed to create IP pool: %v", err)
	}

	// All possible IPs that can be generated
	// This is feasible for small test pools
	allPossibleIPs := map[string]bool{
		"172.16.0.1": true, "172.16.0.2": true,
		"172.16.0.4": true, "172.16.0.5": true,
		"172.16.0.6": true,
		"fd00::1":    true, "fd00::2": true,
		"fd00::4": true, "fd00::5": true,
		"fd00::6": true,
	}
	if pool.total.Cmp(big.NewInt(int64(len(allPossibleIPs)))) != 0 {
		t.Fatalf("Calculated total IPs %s does not match expected %d", pool.total.String(), len(allPossibleIPs))
	}

	counts := make(map[string]int)
	numSamples := int(pool.total.Int64()) * 20 // Sample enough to likely hit all IPs
	if numSamples < 100 {
		numSamples = 100 // Minimum samples
	}

	for i := 0; i < numSamples; i++ {
		ip := pool.GetRandomly()
		if ip == nil {
			t.Fatalf("GetRandomly() returned nil IP on iteration %d", i)
		}
		ipStr := ip.String()
		if _, ok := allPossibleIPs[ipStr]; !ok {
			t.Errorf("GetRandomly() returned IP %s which is not in the set of all possible IPs", ipStr)
		}
		counts[ipStr]++
	}

	// Check if all IPs were hit (probabilistic, but likely for enough samples)
	// For very small pools, this should ensure all are hit.
	if len(counts) != len(allPossibleIPs) {
		t.Logf("GetRandomly() did not generate all possible IPs. Generated %d unique IPs out of %d possible. Counts: %v", len(counts), len(allPossibleIPs), counts)
		// This might not be a strict failure for larger pools if numSamples is not >> total IPs.
		// For our small test pool, it should hit all.
		if pool.total.Int64() <= 10 { // Arbitrary threshold for small pools
			t.Errorf("GetRandomly() failed to generate all %d unique IPs; got %d. Counts: %v", len(allPossibleIPs), len(counts), counts)
		}
	}
	t.Logf("Distribution of random IPs: %v", counts)
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
			subnets: []string{"192.168.1.0/30", "10.0.0.5/32", "172.16.0.0/31"},
			// 192.168.1.0/30 -> .1, .2  (weight 2)
			// 10.0.0.5/32    -> .5      (weight 1)
			// 172.16.0.0/31  -> .0, .1  (weight 2)
			expectedIPs: []string{
				"192.168.1.1", "192.168.1.2",
				"10.0.0.5",
				"172.16.0.0", "172.16.0.1",
			},
		},
		{
			name:    "IPv6 specific cases",
			subnets: []string{"2001:db8::/126", "2001:db8:cafe::a/128", "2001:db8:feed::/127"},
			// 2001:db8::/126     -> ::1, ::2     (weight 2)
			// 2001:db8:cafe::a/128 -> ::a         (weight 1)
			// 2001:db8:feed::/127  -> ::0, ::1    (weight 2)
			expectedIPs: []string{
				"2001:db8::1", "2001:db8::2",
				"2001:db8:cafe::a",
				"2001:db8:feed::", "2001:db8:feed::1",
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
			if err != nil {
				t.Fatalf("NewIpPool failed: %v", err)
			}

			if pool.total.Cmp(big.NewInt(int64(len(tc.expectedIPs)))) != 0 {
				t.Errorf("Expected total %d IPs, but pool calculated %s", len(tc.expectedIPs), pool.total.String())
			}

			var generatedIPs []string
			for i := int64(0); i < pool.total.Int64(); i++ {
				ip := pool.ipFromIndex(big.NewInt(i))
				if ip == nil {
					t.Errorf("ipFromIndex(%d) returned nil", i)
					continue
				}
				generatedIPs = append(generatedIPs, ip.String())
			}

			// Convert expectedIPs to net.IP then string to ensure canonical form for comparison
			canonicalExpectedIPs := make([]string, len(tc.expectedIPs))
			for i, ipStr := range tc.expectedIPs {
				canonicalExpectedIPs[i] = net.ParseIP(ipStr).String()
			}

			if !reflect.DeepEqual(generatedIPs, canonicalExpectedIPs) {
				t.Errorf("Generated IPs mismatch:\nGot:      %v\nExpected: %v", generatedIPs, canonicalExpectedIPs)
			}
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
		{"ipv4 /30", "192.168.0.0/30", big.NewInt(2)},
		{"ipv4 /24", "192.168.0.0/24", big.NewInt(254)},
		{"ipv4 /16", "10.0.0.0/16", big.NewInt(65534)}, // 2^16 - 2
		{"ipv4 /8", "10.0.0.0/8", new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 24), big.NewInt(2))},

		{"ipv6 /128", "2001:db8::1/128", big.NewInt(1)},
		{"ipv6 /127", "2001:db8::/127", big.NewInt(2)},
		{"ipv6 /126", "2001:db8::/126", big.NewInt(2)},
		{"ipv6 /120", "2001:db8::/120", big.NewInt(254)}, // 2^8 - 2
		{"ipv6 /64", "2001:db8:cafe::/64", new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 64), big.NewInt(2))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ipNet, err := net.ParseCIDR(tt.cidr)
			if err != nil {
				t.Fatalf("Failed to parse CIDR %s: %v", tt.cidr, err)
			}
			got := calculateUsableIPs(ipNet)
			if got.Cmp(tt.expected) != 0 {
				t.Errorf("calculateUsableIPs for %s = %s, want %s", tt.cidr, got.String(), tt.expected.String())
			}
		})
	}
}
