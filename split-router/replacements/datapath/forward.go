package datapath

import (
	"io"
	"net"
	"strconv"
	"time"

	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
	"gvisor.dev/gvisor/pkg/waiter"
)

const (
	maxInFlightTCP = 512
	defaultRcvWnd  = 0
	udpFlowTimeout = 60 * time.Second
)

func installForwarders(netStack *stack.Stack, selector *routeSelector) {
	tcpForwarder := tcp.NewForwarder(netStack, defaultRcvWnd, maxInFlightTCP,
		func(request *tcp.ForwarderRequest) { forwardTCP(request, selector) })
	netStack.SetTransportProtocolHandler(tcp.ProtocolNumber, tcpForwarder.HandlePacket)

	udpForwarder := udp.NewForwarder(netStack,
		func(request *udp.ForwarderRequest) bool { return forwardUDP(request, selector) })
	netStack.SetTransportProtocolHandler(udp.ProtocolNumber, udpForwarder.HandlePacket)
}

func forwardTCP(request *tcp.ForwarderRequest, selector *routeSelector) {
	id := request.ID()

	upstream, _, err := selector.dial("tcp", destinationOf(id))
	if err != nil {
		request.Complete(true)
		return
	}

	var queue waiter.Queue
	endpoint, tcpipErr := request.CreateEndpoint(&queue)
	if tcpipErr != nil {
		upstream.Close()
		request.Complete(true)
		return
	}
	request.Complete(false)

	client := gonet.NewTCPConn(&queue, endpoint)
	go relay(client, upstream)
}

func forwardUDP(request *udp.ForwarderRequest, selector *routeSelector) bool {
	id := request.ID()

	upstream, _, err := selector.dial("udp", destinationOf(id))
	if err != nil {
		return false
	}

	var queue waiter.Queue
	endpoint, tcpipErr := request.CreateEndpoint(&queue)
	if tcpipErr != nil {
		upstream.Close()
		return false
	}

	client := gonet.NewUDPConn(&queue, endpoint)
	observeDNS := id.LocalPort == 53
	go relayDatagrams(client, upstream, selector, observeDNS)
	return true
}

func relay(client, upstream net.Conn) {
	defer client.Close()
	defer upstream.Close()

	done := make(chan struct{}, 2)
	go func() {
		_, _ = io.Copy(upstream, client)
		done <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(client, upstream)
		done <- struct{}{}
	}()
	<-done
}

func relayDatagrams(client, upstream net.Conn, selector *routeSelector, observeDNS bool) {
	defer client.Close()
	defer upstream.Close()

	done := make(chan struct{}, 2)
	go func() {
		copyDatagrams(upstream, client, nil)
		done <- struct{}{}
	}()
	go func() {
		var observer func([]byte)
		if observeDNS {
			observer = selector.observeDNSResponse
		}
		copyDatagrams(client, upstream, observer)
		done <- struct{}{}
	}()
	<-done
}

func copyDatagrams(dst, src net.Conn, observer func([]byte)) {
	buffer := make([]byte, maxDatagramSize)
	for {
		if err := src.SetReadDeadline(time.Now().Add(udpFlowTimeout)); err != nil {
			return
		}
		read, err := src.Read(buffer)
		if read > 0 {
			if observer != nil {
				observer(buffer[:read])
			}
			if _, writeErr := dst.Write(buffer[:read]); writeErr != nil {
				return
			}
		}
		if err != nil {
			return
		}
	}
}

const maxDatagramSize = 65535

func destinationOf(id stack.TransportEndpointID) string {
	return net.JoinHostPort(
		addressString(id.LocalAddress),
		strconv.Itoa(int(id.LocalPort)),
	)
}

func addressString(address tcpip.Address) string {
	return net.IP(address.AsSlice()).String()
}
