package protocol

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"net"
)

// Addr defines a SOCKS5 address-port schema. Such struct packages
// hostname/IP and a uint16 port number. IPv4, IPv6, and FQDN is accepted. If
// both IP and FQDN is available, IP address is preferred.
// 		+------+----------+------+
//		| ATYP |   ADDR   | PORT |
//		+------+----------+------+
//		|  1   | Variable |  2   |
//		+------+----------+------+
// In an address field (DST.ADDR, BND.ADDR), the ATYP field specifies the type
// of address contained within the field:
// 		- 0x01: the address is a version-4 IP address, with a length of 4
// 				octets
// 		- 0x03: the address field contains a fully-qualified domain name. The
// 				first octet of the address field contains the number of octets
// 				of name that follow, there is no terminating NUL octet. the
// 				address field contains a fully-qualified domain name. The first
//				octet of the address field contains the number of octets of
// 				name that follow, there is no terminating NUL octet.
// 		- 0x04: the address is a version-6 IP address, with a length of 16
// 				octets.
type Addr struct {
	IP     net.IP
	Domain string
	Port   uint16
}

// NewAddrFromBuffer parse a byte array containing a SOCKS5 address-port
// schema specified in RFC1928, and returns a Addr struct.
func NewAddrFromBuffer(buf []byte) (*Addr, error) {
	return NewAddrFromReader(bytes.NewReader(buf))
}

// NewAddrFromBuffer consume a reader containing bytes in a SOCKS5
// address-port schema specified in RFC1928, and returns a Addr struct.
func NewAddrFromReader(rd io.Reader) (*Addr, error) {
	addr := &Addr{}

	atyp := []byte{0}

	if _, err := rd.Read(atyp); err != nil {
		return nil, err
	}

	switch atyp[0] {
	case AtypIPv4:
		ip := make([]byte, 4)
		if _, err := io.ReadAtLeast(rd, ip, 4); err != nil {
			return nil, err
		}
		addr.IP = ip
	case AtypIPv6:
		ip := make([]byte, 16)
		if _, err := io.ReadAtLeast(rd, ip, 16); err != nil {
			return nil, err
		}
		addr.IP = ip
	case AtypDomain:
		domainLen := []byte{0}
		if _, err := rd.Read(domainLen); err != nil {
			return nil, err
		}
		domain := make([]byte, int(domainLen[0]))
		if _, err := io.ReadAtLeast(rd, domain, int(domainLen[0])); err != nil {
			return nil, err
		}
		addr.Domain = string(domain)

		// some SOCKS5 clients, like Proxy SwitchyOmega don't necessarily follow the spec
		// ip addresses are sometimes encoded as string, served as a domain name with atyp == 0x03
		// though it doesn't hurt, why not just correct them
		if ip := net.ParseIP(addr.Domain); ip != nil {
			addr.IP = ip
		}

	default:
		return nil, fmt.Errorf("unsupported address type %#xs", atyp[0])
	}

	port := []byte{0, 0}
	if _, err := io.ReadAtLeast(rd, port, 2); err != nil {
		return nil, err
	}

	addr.Port = (uint16(port[0]) << 8) | uint16(port[1])

	return addr, nil
}

// NewAddrFromString read a string of a network address of the form "host:port",
// and returns a Addr struct.
func NewAddrFromString(s string) (*Addr, error) {
	hostStr, portStr, err := net.SplitHostPort(s)
	if err != nil {
		return nil, err
	}

	addr := &Addr{}

	if ip := net.ParseIP(hostStr); ip != nil {
		addr.IP = ip
	} else {
		addr.Domain = hostStr
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, err
	}

	addr.Port = uint16(port)
	return addr, err
}

// Marshal encode a Addr struct into byte array suitable for SOCKS5 protocol.
func (addr Addr) Marshal() ([]byte, error) {
	var builder bytes.Buffer

	if addr.IP != nil {
		// prefer IP over FQDN
		if len(addr.IP) == 4 {
			builder.WriteByte(AtypIPv4)
			builder.Write(addr.IP.To4())
		} else {
			builder.WriteByte(AtypIPv6)
			builder.Write(addr.IP.To16())
		}
	} else if addr.Domain != "" {
		builder.WriteByte(AtypDomain)
		builder.WriteByte(byte(len(addr.Domain)))
		builder.WriteString(addr.Domain)
	} else {
		return nil, errors.New("no IP or domain specified")
	}

	builder.WriteByte(byte(addr.Port >> 8))
	builder.WriteByte(byte(addr.Port & 0xff))

	return builder.Bytes(), nil
}

// String implements String function of Stringer interface.
func (addr Addr) String() string {
	if addr.IP != nil {
		// prefer IP over FQDN
		return net.JoinHostPort(addr.IP.String(), strconv.Itoa(int(addr.Port)))
	} else if addr.Domain != "" {
		return net.JoinHostPort(addr.Domain, strconv.Itoa(int(addr.Port)))
	} else {
		return "no IP or domain specified"
	}
}
