package datapath

import (
	"fmt"

	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/link/fdbased"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv6"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/icmp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
)

const nicID tcpip.NICID = 1

type Session struct {
	stack    *stack.Stack
	selector *routeSelector
}

func Start(tunFD int, mtu int) (*Session, error) {
	netStack := stack.New(stack.Options{
		NetworkProtocols: []stack.NetworkProtocolFactory{
			ipv4.NewProtocol,
			ipv6.NewProtocol,
		},
		TransportProtocols: []stack.TransportProtocolFactory{
			tcp.NewProtocol,
			udp.NewProtocol,
			icmp.NewProtocol4,
			icmp.NewProtocol6,
		},
	})

	endpoint, err := fdbased.New(&fdbased.Options{
		FDs: []int{tunFD},
		MTU: uint32(mtu),
	})
	if err != nil {
		netStack.Close()
		return nil, fmt.Errorf("datapath.Start: link endpoint over fd %d: %w", tunFD, err)
	}

	if tcpipErr := netStack.CreateNIC(nicID, endpoint); tcpipErr != nil {
		netStack.Close()
		return nil, fmt.Errorf("datapath.Start: create NIC on fd %d: %v", tunFD, tcpipErr)
	}

	netStack.SetPromiscuousMode(nicID, true)
	netStack.SetSpoofing(nicID, true)
	netStack.SetRouteTable([]tcpip.Route{
		{Destination: header.IPv4EmptySubnet, NIC: nicID},
		{Destination: header.IPv6EmptySubnet, NIC: nicID},
	})

	selector := newRouteSelector()
	installForwarders(netStack, selector)
	return &Session{stack: netStack, selector: selector}, nil
}

func (s *Session) SetNetworks(wifiHandle int64, cellularHandle int64) {
	if s.selector == nil {
		return
	}
	s.selector.setNetworks(uint64(wifiHandle), uint64(cellularHandle))
}

func (s *Session) SetNetwork(handle int64) {
	s.SetNetworks(handle, handle)
}

func (s *Session) Stop() {
	if s.stack == nil {
		return
	}
	s.stack.Close()
	s.stack = nil
}
