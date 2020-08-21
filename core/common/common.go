// Copyright (c) 2016 shawn1m. All rights reserved.
// Use of this source code is governed by The MIT License (MIT) that can be
// found in the LICENSE file.

// Package common provides common functions.
package common

import (
	"net"
	"regexp"
	"strings"

	"github.com/miekg/dns"
	log "github.com/sirupsen/logrus"
)

var ReservedIPNetworkList = getReservedIPNetworkList()

func IsDomainMatchRule(pattern string, domain string) bool {
	matched, err := regexp.MatchString(pattern, domain)
	if err != nil {
		log.Warnf("Error matching domain %s with pattern %s: %s", domain, pattern, err)
	}
	return matched
}

func HasAnswer(m *dns.Msg) bool { return m != nil && len(m.Answer) != 0 }

func HasSubDomain(s string, sub string) bool {
	return strings.HasSuffix(sub, "."+s) || s == sub
}

func getReservedIPNetworkList() *IPSet {
	var ipNetList []*net.IPNet
	localCIDR := []string{"127.0.0.0/8", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "100.64.0.0/10"}
	for _, c := range localCIDR {
		_, ipNet, err := net.ParseCIDR(c)
		if err != nil {
			break
		}
		ipNetList = append(ipNetList, ipNet)
	}
	return NewIPSet(ipNetList)
}

func FindRecordByType(msg *dns.Msg, t uint16) string {
	if msg == nil {
		return ""
	}
	for _, rr := range msg.Answer {
		if rr.Header().Rrtype == t {
			items := strings.SplitN(rr.String(), "\t", 5)
			return items[4]
		}
	}

	return ""
}

func SetMinimumTTL(msg *dns.Msg, minimumTTL uint32) {
	if minimumTTL == 0 {
		return
	}
	for _, a := range msg.Answer {
		if a.Header().Ttl < minimumTTL {
			a.Header().Ttl = minimumTTL
		}
	}
}

func SetTTLByMap(msg *dns.Msg, domainTTLMap map[string]uint32) {
	if len(domainTTLMap) == 0 {
		return
	}
	for _, a := range msg.Answer {
		name := a.Header().Name[:len(a.Header().Name)-1]
		for k, v := range domainTTLMap {
			if IsDomainMatchRule(k, name) {
				a.Header().Ttl = v
			}
		}
	}
}

func EmptyDNSMsg(query *dns.Msg) *dns.Msg {
	msg := new(dns.Msg)
	soa, _ := dns.NewRR(query.Question[0].Name + " IN SOA ns.local. hostmaster.local. 1 7200 3600 1209600 3600")
	msg.Ns = append(msg.Ns, soa)
	msg.SetReply(query)
	msg.RecursionAvailable = true
	return msg
}

func IsEmptyAndNoSOA(q, msg *dns.Msg) bool {
	qtype := q.Question[0].Qtype
	for _, i := range msg.Answer {
		if i.Header().Rrtype == qtype {
			return false
		}
	}
	for _, i := range msg.Ns {
		if i.Header().Rrtype == dns.TypeSOA {
			return false
		}
	}
	return true
}

func HasSOA(msg *dns.Msg) bool {
	for _, i := range msg.Ns {
		if i.Header().Rrtype == dns.TypeSOA {
			return true
		}
	}
	return false
}

func HasType(msg *dns.Msg, qtype uint16) bool {
	for _, i := range msg.Answer {
		if i.Header().Rrtype == qtype {
			return true
		}
	}
	return false
}
