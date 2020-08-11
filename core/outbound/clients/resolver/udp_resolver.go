package resolver

import (
	log "github.com/sirupsen/logrus"

	"github.com/miekg/dns"
	"github.com/shawn1m/overture/core/common"
)

type UDPResolver struct {
	BaseResolver
	tcp Resolver
}

func (r *UDPResolver) Exchange(q *dns.Msg) (*dns.Msg, error) {
	msg, err := r.BaseResolver.Exchange(q)
	if err != nil {
		return nil, err
	}
	if msg.Truncated {
		log.Debugf("truncated msg: %s", q)
		msg, err = r.tcp.Exchange(q)
	}
	return msg, err
}

func (r *UDPResolver) Init() error {
	err := r.BaseResolver.Init()
	if err != nil {
		return err
	}
	tcpupstream := &common.DNSUpstream{
		Name:             r.dnsUpstream.Name + " - tcp",
		Address:          r.dnsUpstream.Address,
		Protocol:         "tcp",
		SOCKS5Address:    r.dnsUpstream.SOCKS5Address,
		Timeout:          r.dnsUpstream.Timeout,
		EDNSClientSubnet: r.dnsUpstream.EDNSClientSubnet,
	}
	r.tcp = NewResolver(tcpupstream)
	return r.tcp.Init()
}
