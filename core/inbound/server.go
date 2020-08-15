// Package inbound implements dns server for inbound connection.
package inbound

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/miekg/dns"
	log "github.com/sirupsen/logrus"

	"github.com/shawn1m/overture/core/common"
	"github.com/shawn1m/overture/core/matcher"
	"github.com/shawn1m/overture/core/outbound"
	"github.com/shawn1m/overture/core/querylog"
	"github.com/shawn1m/overture/replace"
)

type Server struct {
	bindAddress      []string
	debugHttpAddress string
	dispatcher       outbound.Dispatcher
	rejectQType      []uint16
	HTTPMux          *http.ServeMux
	ctx              context.Context
	cancel           context.CancelFunc

	blockDomainList   matcher.Matcher
	blockIPList       *common.IPSet
	replaceDomainList *replace.DomainReplace
	replaceIPList     *replace.IPReplace
}

func NewServer(bindAddress []string, debugHTTPAddress string, dispatcher outbound.Dispatcher, rejectQType []uint16, blockDomainList matcher.Matcher, blockIPList *common.IPSet, replaceDomainList *replace.DomainReplace, replaceIPList *replace.IPReplace) *Server {
	s := &Server{
		bindAddress:       bindAddress,
		debugHttpAddress:  debugHTTPAddress,
		dispatcher:        dispatcher,
		rejectQType:       rejectQType,
		blockDomainList:   blockDomainList,
		blockIPList:       blockIPList,
		replaceDomainList: replaceDomainList,
		replaceIPList:     replaceIPList,
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.HTTPMux = http.NewServeMux()
	return s
}

func (s *Server) DumpCache(w http.ResponseWriter, req *http.Request) {
	if s.dispatcher.Cache == nil {
		io.WriteString(w, "error: cache not enabled")
		return
	}

	type answer struct {
		Name  string `json:"name"`
		TTL   int    `json:"ttl"`
		Type  string `json:"type"`
		Rdata string `json:"rdata"`
	}

	type response struct {
		Length   int                  `json:"length"`
		Capacity int                  `json:"capacity"`
		Body     map[string][]*answer `json:"body"`
	}

	query := req.URL.Query()
	nobody := true
	if t := query.Get("nobody"); strings.ToLower(t) == "false" {
		nobody = false
	}

	rs, l := s.dispatcher.Cache.Dump(nobody)
	body := make(map[string][]*answer)

	for k, es := range rs {
		var answers []*answer
		for _, e := range es {
			ts := strings.Split(e, "\t")
			ttl, _ := strconv.Atoi(ts[1])
			r := &answer{
				Name:  ts[0],
				TTL:   ttl,
				Type:  ts[3],
				Rdata: ts[4],
			}
			answers = append(answers, r)
		}
		body[strings.TrimSpace(k)] = answers
	}

	res := response{
		Body:     body,
		Length:   l,
		Capacity: s.dispatcher.Cache.Capacity(),
	}

	responseBytes, err := json.Marshal(&res)
	if err != nil {
		io.WriteString(w, err.Error())
		return
	}

	io.WriteString(w, string(responseBytes))
}

func (s *Server) Run() {

	mux := dns.NewServeMux()
	mux.Handle(".", s)

	wg := new(sync.WaitGroup)
	wg.Add(2 * len(s.bindAddress))

	log.Infof("Overture is listening on %s", s.bindAddress)

	for _, a := range s.bindAddress {
		for _, p := range [2]string{"tcp", "udp"} {
			go func(p string, a string) {

				// Manual create server inorder to have a way to close it.
				srv := &dns.Server{Addr: a, Net: p, Handler: mux}
				go func() {
					<-s.ctx.Done()
					log.Warnf("Shutting down the server on protocol %s", p)
					srv.ShutdownContext(s.ctx)
				}()
				err := srv.ListenAndServe()
				if err != nil {
					log.Fatalf("Listening on port %s failed: %s", p, err)
					os.Exit(1)
				}
				wg.Done()
			}(p, a)
		}
	}

	if s.debugHttpAddress != "" {
		s.HTTPMux.HandleFunc("/cache", s.DumpCache)
		s.HTTPMux.HandleFunc("/debug/pprof/", pprof.Index)
		s.HTTPMux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		s.HTTPMux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		s.HTTPMux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		s.HTTPMux.HandleFunc("/debug/pprof/trace", pprof.Trace)

		wg.Add(1)
		go func() {
			// Manual create server inorder to have a way to close it.
			srv := &http.Server{
				Addr:    s.debugHttpAddress,
				Handler: s.HTTPMux,
			}
			go func() {
				<-s.ctx.Done()
				log.Warnf("Shutting down debug HTTP server")
				srv.Shutdown(s.ctx)
			}()

			err := srv.ListenAndServe()
			if err != http.ErrServerClosed {
				log.Fatalf("Debug HTTP Server Listen on port %s  faild: %s", s.debugHttpAddress, err)
				os.Exit(1)
			}
			wg.Done()
		}()
	}

	wg.Wait()
}

func (s *Server) Stop() {
	s.cancel()
}

func (s *Server) ServeDNS(w dns.ResponseWriter, q *dns.Msg) {
	inboundIP, _, _ := net.SplitHostPort(w.RemoteAddr().String())

	log.Debugf("Question from %s: %s", inboundIP, q.Question[0].String())

	for _, qt := range s.rejectQType {
		if isQuestionType(q, qt) {
			log.Debugf("Reject %s: %s", inboundIP, q.Question[0].String())
			querylog.Log(inboundIP, q, "Block")
			dns.HandleFailed(w, q)
			return
		}
	}

	qCopy := q
	replaceDomain := s.replaceDomainList.Find(q.Question[0].Name)
	if replaceDomain != "" {
		log.Debugf("replace domain: %s -> %s", q.Question[0].Name, replaceDomain)
		qCopy = q.Copy()
		qCopy.Question[0].Name = replaceDomain + "."
	}

	var responseMessage *dns.Msg
	if s.isBlockDomain(q) {
		responseMessage = common.EmptyDNSMsg(q)
		log.Debugf("Block %s: %s", inboundIP, q.Question[0].String())
	} else {
		responseMessage = s.dispatcher.Exchange(qCopy, inboundIP)
	}

	if responseMessage == nil {
		dns.HandleFailed(w, q)
		return
	}

	// 复制一份，避免修改原始对象
	responseMessage = responseMessage.Copy()

	var answer []dns.RR
	for _, i := range responseMessage.Answer {
		var ip net.IP
		if i.Header().Rrtype == dns.TypeA {
			ip = net.ParseIP(i.(*dns.A).A.String())
		} else if i.Header().Rrtype == dns.TypeAAAA {
			ip = net.ParseIP(i.(*dns.AAAA).AAAA.String())
		}
		if s.blockIPList.Contains(ip, false, "block") {
			log.Debugf("block IP: %s - %s - %s", inboundIP, q.Question[0].Name, ip)
			continue
		}
		answer = append(answer, i)
	}
	responseMessage.Answer = answer

	// 在结果中还原被替换的域名
	responseMessage.SetReply(q)
	for _, i := range responseMessage.Answer {
		i.Header().Name = q.Question[0].Name
	}
	for _, i := range responseMessage.Ns {
		i.Header().Name = q.Question[0].Name
	}
	for _, i := range responseMessage.Extra {
		i.Header().Name = q.Question[0].Name
	}

	var replaceIP net.IP
	for _, i := range responseMessage.Answer {
		var ip net.IP
		if i.Header().Rrtype == dns.TypeA {
			ip = net.ParseIP(i.(*dns.A).A.String())
		} else if i.Header().Rrtype == dns.TypeAAAA {
			ip = net.ParseIP(i.(*dns.AAAA).AAAA.String())
		} else {
			continue
		}
		replaceIP = s.replaceIPList.Find(ip)
		if replaceIP != nil {
			break
		}
	}
	if replaceIP != nil {
		log.Debugf("replace IP: %s -> %s", q.Question[0].Name, replaceIP)
		var rr dns.RR
		if replaceIP.To4() != nil {
			rr, _ = dns.NewRR(responseMessage.Question[0].Name + " IN A " + replaceIP.String())
		} else {
			rr, _ = dns.NewRR(responseMessage.Question[0].Name + " IN AAAA " + replaceIP.String())
		}
		replaceMsg := new(dns.Msg)
		replaceMsg.Answer = []dns.RR{rr}
		replaceMsg.SetReply(q)
		replaceMsg.RecursionAvailable = true
		responseMessage = replaceMsg
	}

	err := w.WriteMsg(responseMessage)
	if err != nil {
		log.Warnf("Write message failed, message: %s, error: %s", responseMessage, err)
		return
	}
}

func isQuestionType(q *dns.Msg, qt uint16) bool { return q.Question[0].Qtype == qt }

func (s *Server) isBlockDomain(query *dns.Msg) bool {
	name := query.Question[0].Name
	name = name[:len(name)-1]
	return s.blockDomainList.Has(name)
}
