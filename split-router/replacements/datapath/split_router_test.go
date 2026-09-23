package datapath

import (
	"encoding/binary"
	"testing"
)

func TestCellularDomains(t *testing.T) {
	for _, name := range []string{"x.com", "api.x.com", "twitter.com", "pbs.twimg.com", "t.co"} {
		if !isCellularDomain(name) {
			t.Fatalf("expected %q to use cellular", name)
		}
	}
	for _, name := range []string{"example.com", "notx.com", "x.com.example.org", "github.com"} {
		if isCellularDomain(name) {
			t.Fatalf("did not expect %q to use cellular", name)
		}
	}
}

func TestDNSResponseMarksXAddressCellular(t *testing.T) {
	selector := newRouteSelector()
	msg := dnsAResponse("x.com", [4]byte{203, 0, 113, 9})
	selector.observeDNSResponse(msg)

	if got := selector.routeForAddress("203.0.113.9:443"); got != routeCellular {
		t.Fatalf("X address route = %v, want cellular", got)
	}
	if got := selector.routeForAddress("203.0.113.10:443"); got != routeWiFi {
		t.Fatalf("unknown address route = %v, want wifi", got)
	}
}

func TestNonXDNSDoesNotMarkAddress(t *testing.T) {
	selector := newRouteSelector()
	selector.observeDNSResponse(dnsAResponse("example.com", [4]byte{203, 0, 113, 9}))
	if got := selector.routeForAddress("203.0.113.9:443"); got != routeWiFi {
		t.Fatalf("non-X address route = %v, want wifi", got)
	}
}

func dnsAResponse(name string, ip [4]byte) []byte {
	msg := make([]byte, 12)
	binary.BigEndian.PutUint16(msg[0:2], 0x1234)
	binary.BigEndian.PutUint16(msg[2:4], 0x8180)
	binary.BigEndian.PutUint16(msg[4:6], 1)
	binary.BigEndian.PutUint16(msg[6:8], 1)

	for _, label := range splitLabels(name) {
		msg = append(msg, byte(len(label)))
		msg = append(msg, label...)
	}
	msg = append(msg, 0)
	msg = append(msg, 0, 1, 0, 1)

	msg = append(msg, 0xC0, 0x0C)
	msg = append(msg, 0, 1, 0, 1)
	msg = append(msg, 0, 0, 0, 60)
	msg = append(msg, 0, 4)
	msg = append(msg, ip[:]...)
	return msg
}

func splitLabels(name string) []string {
	var labels []string
	start := 0
	for i := 0; i <= len(name); i++ {
		if i == len(name) || name[i] == '.' {
			labels = append(labels, name[start:i])
			start = i + 1
		}
	}
	return labels
}
