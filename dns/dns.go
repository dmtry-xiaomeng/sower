package dns

import (
	"net"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/miekg/dns"
	"github.com/wweir/sower/conf"
)

func StartDNS(dnsServer string) {
	dns.HandleFunc(".", func(w dns.ResponseWriter, r *dns.Msg) {
		// *Msg r has an TSIG record and it was validated
		if r.IsTsig() != nil && w.TsigStatus() == nil {
			r.SetTsig(r.Extra[len(r.Extra)-1].(*dns.TSIG).Hdr.Name, dns.HmacMD5, 300, time.Now().Unix())
		}

		//https://stackoverflow.com/questions/4082081/requesting-a-and-aaaa-records-in-single-dns-query/4083071#4083071
		if len(r.Question) == 0 {
			return
		}

		if len(conf.Conf.BlockList) == 0 {
			bestTry(w, r, r.Question[0].Name, dnsServer)
		} else {
			manual(w, r, r.Question[0].Name, dnsServer)
		}
	})

	server := &dns.Server{Addr: ":53", Net: "udp"}
	glog.Fatalln(server.ListenAndServe())
}

func bestTry(w dns.ResponseWriter, r *dns.Msg, domain, dnsServer string) {
	msg, _ := dns.Exchange(r, dnsServer+":53")
	if len(msg.Answer) == 0 { // expose any response
		w.WriteMsg(msg)
		return
	}

	var ip string
	switch msg.Answer[0].(type) {
	case *dns.A:
		ip = msg.Answer[0].(*dns.A).A.String()
	case *dns.AAAA:
		ip = "[" + msg.Answer[0].(*dns.AAAA).AAAA.String() + "]"
	default:
		w.WriteMsg(msg)
	}

	if _, err := net.DialTimeout("tcp", ip+":http", time.Second); err != nil {
		glog.V(2).Infoln(ip+":80", err)
		w.WriteMsg(localA(r, domain))
		return
	}
	w.WriteMsg(msg)
}

func manual(w dns.ResponseWriter, r *dns.Msg, domain, dnsServer string) {
	if rule.Match(strings.TrimSuffix(domain, ".")) {
		glog.V(2).Infof("match %s suss", domain)
		w.WriteMsg(localA(r, domain))
		return
	}
	glog.V(2).Infof("match %s fail", domain)

	// expose any response
	msg, _ := dns.Exchange(r, dnsServer+":53")
	w.WriteMsg(msg)
}

func localA(r *dns.Msg, domain string) *dns.Msg {
	m := new(dns.Msg)
	m.SetReply(r)
	m.Answer = []dns.RR{&dns.A{
		Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 720},
		A:   conf.Conf.ClientIPNet,
	}}
	return m
}