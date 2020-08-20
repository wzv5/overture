package common

import (
	"net"

	"github.com/miekg/dns"
)

type EDNSClientSubnetType struct {
	Policy     string
	ExternalIP string
	NoCookie   bool
}

func SetEDNSClientSubnet(m *dns.Msg, ip string) {
	if ip == "" {
		return
	}

	o := m.IsEdns0()
	if o == nil {
		o = new(dns.OPT)
		o.SetUDPSize(4096)
		o.Hdr.Name = "."
		o.Hdr.Rrtype = dns.TypeOPT
		m.Extra = append(m.Extra, o)
	}

	es := isEDNSClientSubnet(o)
	if es == nil {
		es = new(dns.EDNS0_SUBNET)
		es.Code = dns.EDNS0SUBNET
		es.Address = net.ParseIP(ip)
		if es.Address.To4() != nil {
			es.Family = 1         // 1 for IPv4 source address, 2 for IPv6
			es.SourceNetmask = 16 // 32 for IPV4, 128 for IPv6
		} else {
			es.Family = 2         // 1 for IPv4 source address, 2 for IPv6
			es.SourceNetmask = 56 // 32 for IPV4, 128 for IPv6
		}
		es.SourceScope = 0
		o.Option = append(o.Option, es)
	}
}

func DeleteCookie(m *dns.Msg) {
	o := m.IsEdns0()
	if o == nil {
		return
	}
	for i, e0 := range o.Option {
		switch e0.(type) {
		case *dns.EDNS0_COOKIE:
			o.Option = append(o.Option[:i], o.Option[i+1:]...)
		}
	}
}

func isEDNSClientSubnet(o *dns.OPT) *dns.EDNS0_SUBNET {
	for _, s := range o.Option {
		switch e := s.(type) {
		case *dns.EDNS0_SUBNET:
			return e
		}
	}
	return nil
}

func GetEDNSClientSubnetIP(m *dns.Msg) string {
	o := m.IsEdns0()
	if o != nil {
		for _, s := range o.Option {
			switch e := s.(type) {
			case *dns.EDNS0_SUBNET:
				return e.Address.String()
			}
		}
	}
	return ""
}
