/*
 * Copyright (c) 2019 shawn1m. All rights reserved.
 * Use of this source code is governed by The MIT License (MIT) that can be
 * found in the LICENSE file..
 */

package clients

import (
	"strings"

	"github.com/miekg/dns"
	"github.com/shawn1m/overture/core/outbound/clients/resolver"
	log "github.com/sirupsen/logrus"

	"github.com/shawn1m/overture/core/cache"
	"github.com/shawn1m/overture/core/common"
)

type RemoteClientBundle struct {
	responseMessage *dns.Msg
	questionMessage *dns.Msg

	clients []*RemoteClient

	dnsUpstreams []*common.DNSUpstream
	inboundIP    string
	minimumTTL   int
	domainTTLMap map[string]uint32

	cache *cache.Cache
	Name  string

	dnsResolvers []resolver.Resolver
}

func NewClientBundle(q *dns.Msg, ul []*common.DNSUpstream, resolvers []resolver.Resolver, ip string, minimumTTL int, cache *cache.Cache, name string, domainTTLMap map[string]uint32) *RemoteClientBundle {
	cb := &RemoteClientBundle{questionMessage: q.Copy(), dnsUpstreams: ul, dnsResolvers: resolvers, inboundIP: ip, minimumTTL: minimumTTL, cache: cache, Name: name, domainTTLMap: domainTTLMap}

	for i, u := range ul {
		c := NewClient(cb.questionMessage, u, cb.dnsResolvers[i], cb.inboundIP, cb.cache)
		cb.clients = append(cb.clients, c)
	}

	return cb
}

func (cb *RemoteClientBundle) Exchange(isCache bool, isLog bool) *dns.Msg {
	ch := make(chan *RemoteClient, len(cb.clients))

	for _, o := range cb.clients {
		go func(c *RemoteClient, ch chan *RemoteClient) {
			c.Exchange(isLog)
			ch <- c
		}(o, ch)
	}

	var ec, fallbackc1, fallbackc2 *RemoteClient

	for i := 0; i < len(cb.clients); i++ {
		c := <-ch
		if c != nil && c.responseMessage != nil {
			if common.HasType(c.responseMessage, c.questionMessage.Question[0].Qtype) {
				ec = c
				break
			}
			if c.responseMessage.Answer != nil || common.HasSOA(c.responseMessage) {
				fallbackc1 = c
			}
			fallbackc2 = c
			log.Debugf("DNSUpstream %s returned None answer, dropping it and wait the next one", c.dnsUpstream.Address)
		}
	}

	if ec == nil && fallbackc1 != nil {
		ec = fallbackc1
	}
	if ec == nil && fallbackc2 != nil {
		ec = fallbackc2
	}
	if ec != nil {
		cb.responseMessage = ec.responseMessage
		cb.questionMessage = ec.questionMessage

		common.SetMinimumTTL(cb.responseMessage, uint32(cb.minimumTTL))
		common.SetTTLByMap(cb.responseMessage, cb.domainTTLMap)

		if isCache {
			cb.CacheResultIfNeeded()
		}
	} else {
		log.Errorf("All upstream failed: %s %s [%s]", strings.TrimRight(cb.questionMessage.Question[0].Name, "."), dns.Type(cb.questionMessage.Question[0].Qtype).String(), cb.Name)
	}

	return cb.responseMessage
}

func (cb *RemoteClientBundle) ExchangeFromCache() *dns.Msg {
	for _, o := range cb.clients {
		cb.responseMessage = o.ExchangeFromCache()
		if cb.responseMessage != nil {
			return cb.responseMessage
		}
	}
	return cb.responseMessage
}

func (cb *RemoteClientBundle) CacheResultIfNeeded() {
	if cb.cache != nil && cb.responseMessage != nil && !common.IsEmptyAndNoSOA(cb.questionMessage, cb.responseMessage) {
		cb.cache.InsertMessage(cache.Key(cb.questionMessage.Question[0], common.GetEDNSClientSubnetIP(cb.questionMessage)), cb.responseMessage, uint32(cb.minimumTTL))
	}
}

func (cb *RemoteClientBundle) IsType(t uint16) bool {
	return t == cb.questionMessage.Question[0].Qtype
}

func (cb *RemoteClientBundle) GetFirstQuestionDomain() string {
	return cb.questionMessage.Question[0].Name[:len(cb.questionMessage.Question[0].Name)-1]
}

func (cb *RemoteClientBundle) GetResponseMessage() *dns.Msg {
	return cb.responseMessage
}
