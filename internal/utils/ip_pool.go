package utils

import (
	"crypto/sha256"
	"fmt"
	"math/big"
	"math/rand"
	"net"
)

type IpPool struct {
	subnets []*net.IPNet // Subnets in CIDR format
	weights []*big.Int   // Weight for each subnet (num usable IPs)
	total   *big.Int     // Total weight (sum of all usable IPs)
	rnd     *rand.Rand
}

func NewIpPool(subnets []string) (*IpPool, error) {
	pool := &IpPool{
		subnets: make([]*net.IPNet, 0),
		weights: make([]*big.Int, 0),
		total:   big.NewInt(0),
		rnd:     rand.New(rand.NewSource(int64(rand.Uint64()))),
	}

	for _, subnet := range subnets {
		var ipNet *net.IPNet
		if ip := net.ParseIP(subnet); ip != nil {
			if ip.To4() != nil {
				ipNet = &net.IPNet{IP: ip, Mask: net.CIDRMask(32, 32)}
			} else {
				ipNet = &net.IPNet{IP: ip, Mask: net.CIDRMask(128, 128)}
			}
		} else if _, ipNet2, err := net.ParseCIDR(subnet); err == nil {
			ipNet = ipNet2
		} else {
			return nil, fmt.Errorf("invalid IP or CIDR: %s", subnet)
		}

		numIPs := calculateUsableIPs(ipNet)
		if numIPs.Cmp(big.NewInt(0)) <= 0 {
			continue
		}

		pool.subnets = append(pool.subnets, ipNet)
		pool.weights = append(pool.weights, numIPs)
		pool.total.Add(pool.total, numIPs)
	}

	if len(pool.subnets) == 0 {
		return nil, fmt.Errorf("no valid subnets with usable IPs provided")
	}
	return pool, nil
}

func calculateUsableIPs(ipNet *net.IPNet) *big.Int {
	ones, bits := ipNet.Mask.Size()
	totalIPs := big.NewInt(1)
	totalIPs.Lsh(totalIPs, uint(bits-ones)) // 2^(bits-ones)

	if totalIPs.Cmp(big.NewInt(4)) >= 0 {
		// Exclude network and broadcast addresses
		return big.NewInt(0).Sub(totalIPs, big.NewInt(2))
	}
	// For small subnets (<4 IPs), use all IPs
	return totalIPs
}

func (p *IpPool) GetByKey(key string) net.IP {
	hash := sha256.Sum256([]byte(key))
	hashInt := big.NewInt(0).SetBytes(hash[:])

	index := big.NewInt(0).Mod(hashInt, p.total)
	return p.ipFromIndex(index)
}

func (p *IpPool) ipFromIndex(index *big.Int) net.IP {
	// Find the subnet and offset
	currentIndex := big.NewInt(0)
	for i, weight := range p.weights {
		nextIndex := big.NewInt(0).Add(currentIndex, weight)
		if index.Cmp(currentIndex) >= 0 && index.Cmp(nextIndex) < 0 {
			offset := big.NewInt(0).Sub(index, currentIndex)
			return p.ipFromSubnet(p.subnets[i], offset)
		}
		currentIndex.Set(nextIndex)
	}

	// fallback
	return p.ipFromSubnet(p.subnets[0], big.NewInt(0))
}

func (p *IpPool) ipFromSubnet(subnet *net.IPNet, offset *big.Int) net.IP {
	ones, bits := subnet.Mask.Size()
	totalIPs := big.NewInt(1)
	totalIPs.Lsh(totalIPs, uint(bits-ones))

	// Adjust offset for subnets with >= 4 IPs
	startOffset := big.NewInt(0)
	if totalIPs.Cmp(big.NewInt(4)) >= 0 {
		startOffset = big.NewInt(1)
	}

	// Calculate IP
	isIpv4 := bits == 32
	baseIP := subnet.IP
	ipInt := big.NewInt(0)
	if baseIP.To4() != nil {
		ipInt.SetBytes(baseIP.To4())
	} else {
		ipInt.SetBytes(baseIP)
	}
	ipInt.Add(ipInt, startOffset)
	ipInt.Add(ipInt, offset)

	newIP := make(net.IP, bits/8)
	if isIpv4 {
		bytes := ipInt.Bytes()
		for len(bytes) < 4 {
			bytes = append([]byte{0}, bytes...)
		}
		copy(newIP, bytes[len(bytes)-4:])
	} else {
		bytes := ipInt.Bytes()
		for len(bytes) < 16 {
			bytes = append([]byte{0}, bytes...)
		}
		copy(newIP, bytes[len(bytes)-16:])
	}
	return newIP
}

func (p *IpPool) GetRandomly() net.IP {
	index := big.NewInt(0).Rand(p.rnd, p.total)
	return p.ipFromIndex(index)
}
