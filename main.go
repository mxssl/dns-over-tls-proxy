package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"time"

	"github.com/miekg/dns"
	"github.com/patrickmn/go-cache"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// Handler handles ServeDNS
type Handler struct{}

// Answer is DNS A answer type
type Answer struct {
	ip  string
	ttl uint32
}

// CLI flags
var (
	ip       = flag.String("ip", "0.0.0.0", "IP address")
	port     = flag.String("port", "53", "TCP/UDP Port")
	resolver = flag.String("resolver", "1.1.1.1:853", "DNS-over-TLS resolver")
)

func init() {
	// Setup log format
	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stdout)
	log.SetLevel(log.InfoLevel)
}

// Setup in-memory cache with default expirity and clean time
var inmem = cache.New(1*time.Minute, 1*time.Minute)

func main() {
	// Parse CLI flags
	flag.Parse()

	// Build ip plus port string
	ipPort := fmt.Sprintf("%v:%v", *ip, *port)

	// Setup TCP server
	srvTCP := &dns.Server{Addr: ipPort, Net: "tcp"}

	// Setup UDP server
	srvUDP := &dns.Server{Addr: ipPort, Net: "udp"}

	// Setup handler func
	srvTCP.Handler = Handler{}
	srvUDP.Handler = Handler{}

	// Run TCP server
	go func() {
		if err := srvTCP.ListenAndServe(); err != nil {
			log.Fatal(err)
		}
	}()

	// Run UDP server
	go func() {
		if err := srvUDP.ListenAndServe(); err != nil {
			log.Fatal(err)
		}
	}()

	log.Printf("App is ready to accept connections on %v TCP/UDP", srvTCP.Addr)

	// Setup graceful shutdown channel
	c := make(chan os.Signal, 1)

	// Accept graceful shutdown when quit via SIGINT (Ctrl+C) signal
	signal.Notify(c, os.Interrupt)

	// Block until receive a signal
	<-c

	log.Println("Gracefully shutting down the app...")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	// Wait until the timeout deadline if there are any active connections
	if err := srvTCP.ShutdownContext(ctx); err != nil {
		log.Error(err)
	}

	if err := srvUDP.ShutdownContext(ctx); err != nil {
		log.Error(err)
	}

	os.Exit(0)
}

// ServeDNS handler for DNS inbound queries
func (Handler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	msg := dns.Msg{}
	msg.SetReply(r)

	// Check that DNS request exists
	if len(r.Question) > 0 {

		// Log DNS request
		log.WithFields(log.Fields{
			"remote_addr":      w.RemoteAddr().String(),
			"requested_domain": r.Question[0].Name,
			"protocol":         w.RemoteAddr().Network(),
		}).Info("DNS request")

		// Check question type
		// Only A type is supported for now
		if r.Question[0].Qtype == dns.TypeA {
			domain := msg.Question[0].Name

			// Resolve a domain name
			answer, useCache, err := resolveOverTLS(domain, *resolver)
			if err != nil {
				log.Println(err)
			}
			log.WithFields(log.Fields{
				"use_cache": useCache,
			}).Info("Cache usage")

			// Create a DNS response
			msg.Answer = append(msg.Answer, &dns.A{
				Hdr: dns.RR_Header{Name: domain,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    answer.ttl,
				},
				A: net.ParseIP(answer.ip),
			})

			// Send a response back to a client
			if err := w.WriteMsg(&msg); err != nil {
				log.Error(err)
			}
			return
		}
	}
	log.Error("Not implemented yet")
}

// Resolve domain name by DNS-over-TLS protocol
func resolveOverTLS(domain, dnsServer string) (Answer, bool, error) {
	// Answer type return variable
	var answer Answer

	// Check in-memory cache
	if mem, expr, found := inmem.GetWithExpiration(domain); found {
		answer = mem.(Answer)
		// Calculate DNS TTL time (expiration time - current time)
		answer.ttl = uint32(expr.Sub(time.Now()).Seconds())
		return answer, true, nil
	}

	// Create new dns message
	m := new(dns.Msg)

	// Set type for new dns message
	m.SetQuestion(domain, dns.TypeA)

	// Setup new DNS client
	c := new(dns.Client)

	// Use DNS-over-TLS connection type
	c.Net = "tcp-tls"

	// Make a DNS query
	in, _, err := c.Exchange(m, dnsServer)
	if err != nil {
		return answer, false, errors.Errorf("DNS query error: %v", err)
	}

	// Check that answer exists
	if len(in.Answer) > 0 {
		// Check that DNS response type is A
		if t, ok := in.Answer[0].(*dns.A); ok {
			answer.ip = t.A.String()
			answer.ttl = t.Header().Ttl
		} else {
			return answer, false, errors.New("DNS query error")
		}
	}

	// Set cache record
	inmem.Set(domain, answer, time.Duration(int64(answer.ttl))*time.Second)

	return answer, false, nil
}
