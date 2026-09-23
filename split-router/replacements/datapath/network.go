package datapath

import (
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

const unbound = 0

type routeKind uint8

const (
	routeWiFi routeKind = iota
	routeCellular
)

func (r routeKind) String() string {
	if r == routeCellular {
		return "cellular"
	}
	return "wifi"
}

type routeSelector struct {
	wifi     atomic.Uint64
	cellular atomic.Uint64

	mu          sync.RWMutex
	cellularIPs map[string]struct{}
}

func newRouteSelector() *routeSelector {
	return &routeSelector{cellularIPs: make(map[string]struct{})}
}

func (r *routeSelector) setNetworks(wifi, cellular uint64) {
	r.wifi.Store(wifi)
	r.cellular.Store(cellular)
}

func (r *routeSelector) markCellularIP(ip string) {
	parsed := net.ParseIP(strings.TrimSpace(ip))
	if parsed == nil {
		return
	}
	r.mu.Lock()
	r.cellularIPs[parsed.String()] = struct{}{}
	r.mu.Unlock()
}

func (r *routeSelector) routeForAddress(address string) routeKind {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		host = address
	}

	parsed := net.ParseIP(strings.Trim(host, "[]"))
	if parsed == nil {
		return routeWiFi
	}

	r.mu.RLock()
	_, ok := r.cellularIPs[parsed.String()]
	r.mu.RUnlock()
	if ok {
		return routeCellular
	}
	return routeWiFi
}

func (r *routeSelector) handleFor(route routeKind) uint64 {
	if route == routeCellular {
		return r.cellular.Load()
	}
	return r.wifi.Load()
}

func (r *routeSelector) dial(network, address string) (net.Conn, routeKind, error) {
	route := r.routeForAddress(address)
	handle := r.handleFor(route)
	log.Printf("split-router route=%s network=%s dest=%s", route, network, address)
	if handle == unbound {
		return nil, route, fmt.Errorf("%s upstream unavailable", route)
	}

	dialer := &net.Dialer{
		Timeout: dialTimeout,
		Control: fixedNetworkBinding(handle).control,
	}
	conn, err := dialer.Dial(network, address)
	return conn, route, err
}

type fixedNetworkBinding uint64

func (b fixedNetworkBinding) control(network, address string, conn syscall.RawConn) error {
	handle := uint64(b)
	if handle == unbound {
		return fmt.Errorf("network binding requested with no network handle")
	}

	var bindErr error
	if err := conn.Control(func(fd uintptr) {
		bindErr = bindToNetwork(handle, fd)
	}); err != nil {
		return err
	}
	return bindErr
}

const dialTimeout = 10 * time.Second
