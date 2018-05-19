//
// Perform a DNS query of a given name & type and return an array of
// maps with suitable results.
//

package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/miekg/dns"
)

//
// Here we have a map of types, we only cover the few we care about.
//
var StringToType = map[string]uint16{
	"A":     dns.TypeA,
	"AAAA":  dns.TypeAAAA,
	"CNAME": dns.TypeCNAME,
	"MX":    dns.TypeMX,
	"NS":    dns.TypeNS,
	"PTR":   dns.TypePTR,
	"SOA":   dns.TypeSOA,
	"TXT":   dns.TypeTXT,
}

var (
	localm *dns.Msg
	localc *dns.Client
	conf   *dns.ClientConfig
)

// lookup will perform a DNS query, using the nameservers in /etc/resolv.conf,
// and return an array of maps of the response
//
func lookup(name string, ltype string) ([]map[string]string, error) {

	var results []map[string]string

	var err error
	conf, err = dns.ClientConfigFromFile("/etc/resolv.conf")
	if err != nil || conf == nil {
		fmt.Printf("Cannot initialize the local resolver: %s\n", err)
		os.Exit(1)
	}
	localm = &dns.Msg{
		MsgHdr: dns.MsgHdr{
			RecursionDesired: true,
		},
		Question: make([]dns.Question, 1),
	}
	localc = &dns.Client{
		ReadTimeout: 5 * time.Second,
	}
	r, err := localQuery(dns.Fqdn(name), ltype)
	if err != nil || r == nil {
		return nil, errors.New(fmt.Sprintf("Cannot retrieve the list of name servers for %s\n", name))

	}
	if r.Rcode == dns.RcodeNameError {
		return nil, errors.New(fmt.Sprintf("No such domain %s\n", dns.Fqdn(name)))
	}

	for _, ent := range r.Answer {

		tmp := make(map[string]string)

		tmp["name"] = ent.Header().Name
		tmp["ttl"] = fmt.Sprintf("%d", ent.Header().Ttl)

		//
		// Lookup the value
		//
		switch ent.(type) {
		case *dns.A:
			a := ent.(*dns.A).A
			tmp["type"] = "A"
			tmp["value"] = fmt.Sprintf("%s", a)
		case *dns.AAAA:
			aaaa := ent.(*dns.AAAA).AAAA
			tmp["type"] = "AAAA"
			tmp["value"] = fmt.Sprintf("%s", aaaa)

		case *dns.CNAME:
			cname := ent.(*dns.CNAME).Target
			tmp["type"] = "CNAME"
			tmp["value"] = cname
		case *dns.MX:
			mx_name := ent.(*dns.MX).Mx
			mx_prio := ent.(*dns.MX).Preference
			tmp["type"] = "MX"
			tmp["value"] = fmt.Sprintf("%d\t%s", mx_prio, mx_name)
		case *dns.NS:
			nameserver := ent.(*dns.NS).Ns
			tmp["type"] = "NS"
			tmp["value"] = nameserver
		case *dns.PTR:
			ptr := ent.(*dns.PTR).Ptr
			tmp["type"] = "PTR"
			tmp["value"] = ptr
		case *dns.SOA:
			serial := ent.(*dns.SOA).Serial
			tmp["type"] = "SOA"
			tmp["value"] = fmt.Sprintf("%d", serial)
		case *dns.TXT:
			txt := ent.(*dns.TXT).Txt
			tmp["type"] = "TXT"
			tmp["value"] = fmt.Sprintf("%s", txt[0])
		}
		results = append(results, tmp)

	}
	return results, nil
}

//
// Given a thing to lookup, and a type, do the necessary.
//
// e.g. "steve.fi" "txt"
//
func localQuery(qname string, lookupType string) (*dns.Msg, error) {
	qtype := StringToType[lookupType]
	localm.SetQuestion(qname, qtype)
	for i := range conf.Servers {
		server := conf.Servers[i]
		r, _, err := localc.Exchange(localm, server+":"+conf.Port)
		if err != nil {
			return nil, err
		}
		if r == nil || r.Rcode == dns.RcodeNameError || r.Rcode == dns.RcodeSuccess {
			return r, err
		}
	}
	return nil, errors.New("No name server to answer the question")
}
