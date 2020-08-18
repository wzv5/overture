/*
 * Copyright (c) 2019 shawn1m. All rights reserved.
 * Use of this source code is governed by The MIT License (MIT) that can be
 * found in the LICENSE file..
 */

// Package outbound implements multiple dns client and dispatcher for outbound connection.
package clients

import (
	"fmt"
	"net"
	"sync"

	"github.com/miekg/dns"
	log "github.com/sirupsen/logrus"

	"github.com/shawn1m/overture/core/cache"
	"github.com/shawn1m/overture/core/common"
	"github.com/shawn1m/overture/core/outbound/clients/resolver"
)

type RemoteClient struct {
	responseMessage *dns.Msg
	questionMessage *dns.Msg

	dnsUpstream        *common.DNSUpstream
	ednsClientSubnetIP string
	inboundIP          string
	dnsResolver        resolver.Resolver

	cache *cache.Cache

	lock   sync.Mutex
	reqKey string
}

var pendingRequest sync.Map

func NewClient(q *dns.Msg, u *common.DNSUpstream, resolver resolver.Resolver, ip string, cache *cache.Cache) *RemoteClient {
	c := &RemoteClient{questionMessage: q.Copy(), dnsUpstream: u, dnsResolver: resolver, inboundIP: ip, cache: cache}
	c.getEDNSClientSubnetIP()
	c.reqKey = fmt.Sprintf("%s %d %s %s", q.Question[0].Name, q.Question[0].Qtype, c.ednsClientSubnetIP, u.Name)

	return c
}

func (c *RemoteClient) getEDNSClientSubnetIP() {
	switch c.dnsUpstream.EDNSClientSubnet.Policy {
	case "auto":
		if !common.ReservedIPNetworkList.Contains(net.ParseIP(c.inboundIP), false, "") {
			c.ednsClientSubnetIP = c.inboundIP
		} else {
			c.ednsClientSubnetIP = c.dnsUpstream.EDNSClientSubnet.ExternalIP
		}
	case "manual":
		if c.dnsUpstream.EDNSClientSubnet.ExternalIP != "" &&
			!common.ReservedIPNetworkList.Contains(net.ParseIP(c.dnsUpstream.EDNSClientSubnet.ExternalIP), false, "") {
			c.ednsClientSubnetIP = c.dnsUpstream.EDNSClientSubnet.ExternalIP
			return
		}
	case "disable":
	}
}

func (c *RemoteClient) ExchangeFromCache() *dns.Msg {
	cacheClient := NewCacheClient(c.questionMessage, c.ednsClientSubnetIP, c.cache)
	c.responseMessage = cacheClient.Exchange()
	if c.responseMessage != nil {
		return c.responseMessage
	}
	return nil
}

func (c *RemoteClient) Exchange(isLog bool) *dns.Msg {

	if c1, ok := pendingRequest.LoadOrStore(c.reqKey, c); ok {
		log.Debugf("found pending client %s", c.reqKey)
		msg := c1.(*RemoteClient)._exchange(isLog)
		c.responseMessage = msg
		return msg
	}

	defer pendingRequest.Delete(c.reqKey)
	return c._exchange(isLog)
}

func (c *RemoteClient) _exchange(isLog bool) *dns.Msg {
	c.lock.Lock()
	defer c.lock.Unlock()

	common.SetEDNSClientSubnet(c.questionMessage, c.ednsClientSubnetIP,
		c.dnsUpstream.EDNSClientSubnet.NoCookie)
	c.ednsClientSubnetIP = common.GetEDNSClientSubnetIP(c.questionMessage)

	if c.responseMessage != nil {
		return c.responseMessage
	}

	var temp *dns.Msg
	var err error
	temp, err = c.dnsResolver.Exchange(c.questionMessage)

	if err != nil {
		log.Debugf("%s Fail: %s", c.dnsUpstream.Name, err)
		return nil
	}
	if temp == nil {
		log.Debugf("%s Fail: Response message returned nil, maybe timeout? Please check your query or DNS configuration", c.dnsUpstream.Name)
		return nil
	}

	c.responseMessage = temp

	if isLog {
		c.logAnswer("")
	}

	return c.responseMessage
}

func (c *RemoteClient) logAnswer(indicator string) {

	for _, a := range c.responseMessage.Answer {
		var name string
		if indicator != "" {
			name = indicator
		} else {
			name = c.dnsUpstream.Name
		}
		log.Debugf("Answer from %s: %s", name, a.String())
	}
}
