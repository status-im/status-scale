package cluster

import (
	"net"
	"sync"
)

func NewIPAM(cidr string) (*IPAM, error) {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}
	return &IPAM{
		cidr:  ipnet,
		given: 1, // start from 2
	}, err
}

type IPAM struct {
	cidr *net.IPNet

	mu    sync.Mutex
	given byte
}

// FIXME this will work only for 255 first ips
func (i *IPAM) Take() net.IP {
	i.mu.Lock()
	defer i.mu.Unlock()
	new := make(net.IP, 4)
	copy(new, i.cidr.IP)
	i.given++
	new[3] += i.given
	return new
}

// Peek returns current IP
func (i *IPAM) Peek() net.IP {
	i.mu.Lock()
	defer i.mu.Unlock()
	new := make(net.IP, 4)
	copy(new, i.cidr.IP)
	new[3] += i.given
	return new
}

func (i *IPAM) String() string {
	return i.cidr.String()
}
