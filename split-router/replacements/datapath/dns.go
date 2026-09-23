package datapath

import (
	"encoding/binary"
	"net"
	"strings"
)

var cellularDomainSuffixes = []string{
	"x.com",
	"twitter.com",
	"twimg.com",
	"t.co",
}

func isCellularDomain(name string) bool {
	name = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(name)), ".")
	for _, suffix := range cellularDomainSuffixes {
		if name == suffix || strings.HasSuffix(name, "."+suffix) {
			return true
		}
	}
	return false
}

func (r *routeSelector) observeDNSResponse(msg []byte) {
	if len(msg) < 12 || msg[2]&0x80 == 0 {
		return
	}

	qd := int(binary.BigEndian.Uint16(msg[4:6]))
	an := int(binary.BigEndian.Uint16(msg[6:8]))
	ns := int(binary.BigEndian.Uint16(msg[8:10]))
	ar := int(binary.BigEndian.Uint16(msg[10:12]))

	off := 12
	cellularQuery := false
	for i := 0; i < qd; i++ {
		name, next, ok := dnsName(msg, off)
		if !ok || next+4 > len(msg) {
			return
		}
		if isCellularDomain(name) {
			cellularQuery = true
		}
		off = next + 4
	}

	for i := 0; i < an+ns+ar; i++ {
		_, next, ok := dnsName(msg, off)
		if !ok || next+10 > len(msg) {
			return
		}

		rrtype := binary.BigEndian.Uint16(msg[next : next+2])
		rdlen := int(binary.BigEndian.Uint16(msg[next+8 : next+10]))
		rdata := next + 10
		if rdata+rdlen > len(msg) {
			return
		}

		if cellularQuery {
			switch rrtype {
			case 1:
				if rdlen == net.IPv4len {
					r.markCellularIP(net.IP(msg[rdata : rdata+rdlen]).String())
				}
			case 28:
				if rdlen == net.IPv6len {
					r.markCellularIP(net.IP(msg[rdata : rdata+rdlen]).String())
				}
			}
		}

		off = rdata + rdlen
	}
}

func dnsName(msg []byte, off int) (string, int, bool) {
	if off < 0 || off >= len(msg) {
		return "", off, false
	}

	labels := make([]string, 0, 4)
	cursor := off
	next := -1
	jumps := 0

	for {
		if cursor >= len(msg) || jumps > 32 {
			return "", off, false
		}

		length := int(msg[cursor])
		switch {
		case length == 0:
			cursor++
			if next < 0 {
				next = cursor
			}
			return strings.Join(labels, "."), next, true

		case length&0xC0 == 0xC0:
			if cursor+1 >= len(msg) {
				return "", off, false
			}
			ptr := ((length & 0x3F) << 8) | int(msg[cursor+1])
			if ptr >= len(msg) {
				return "", off, false
			}
			if next < 0 {
				next = cursor + 2
			}
			cursor = ptr
			jumps++

		case length&0xC0 != 0:
			return "", off, false

		default:
			cursor++
			if length == 0 || cursor+length > len(msg) {
				return "", off, false
			}
			labels = append(labels, string(msg[cursor:cursor+length]))
			cursor += length
		}
	}
}
