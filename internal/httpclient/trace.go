package httpclient

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http/httptrace"
	"strings"
	"sync"
	"time"
)

// connTrace records every event net/http reports while a request runs.
type connTrace struct {
	mu     sync.Mutex
	start  time.Time
	events []traceEvent
}

type traceEvent struct {
	elapsed time.Duration
	name    string
	detail  string
}

func newConnTrace() *connTrace {
	return &connTrace{start: time.Now()}
}

func (t *connTrace) add(name, format string, args ...any) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.events = append(t.events, traceEvent{
		elapsed: time.Since(t.start),
		name:    name,
		detail:  fmt.Sprintf(format, args...),
	})
}

func (t *connTrace) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	var b strings.Builder
	for _, e := range t.events {
		fmt.Fprintf(&b, "%8.2fms  %-21s %s\n",
			float64(e.elapsed.Microseconds())/1000, e.name, e.detail)
	}
	return b.String()
}

func withConnTrace(ctx context.Context, t *connTrace) context.Context {
	return httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{
		GetConn: func(hostPort string) {
			t.add("GetConn", "%s", hostPort)
		},
		GotConn: func(i httptrace.GotConnInfo) {
			t.add("GotConn", "remote=%s local=%s reused=%v was_idle=%v idle_time=%s",
				i.Conn.RemoteAddr(), i.Conn.LocalAddr(), i.Reused, i.WasIdle, i.IdleTime)
		},
		DNSStart: func(i httptrace.DNSStartInfo) {
			t.add("DNSStart", "%+v", i)
		},
		DNSDone: func(i httptrace.DNSDoneInfo) {
			t.add("DNSDone", "%+v", i)
		},
		ConnectStart: func(network, addr string) {
			t.add("ConnectStart", "%s %s", network, addr)
		},
		ConnectDone: func(network, addr string, err error) {
			t.add("ConnectDone", "%s %s err=%v", network, addr, err)
		},
		TLSHandshakeStart: func() {
			t.add("TLSHandshakeStart", "")
		},
		TLSHandshakeDone: func(s tls.ConnectionState, err error) {
			t.add("TLSHandshakeDone",
				"version=%s cipher=%s alpn=%q server_name=%s resumed=%v err=%v",
				tls.VersionName(s.Version), tls.CipherSuiteName(s.CipherSuite),
				s.NegotiatedProtocol, s.ServerName, s.DidResume, err)
		},
		WroteHeaderField: func(key string, value []string) {
			t.add("WroteHeaderField", "%s: %s", key, strings.Join(value, ", "))
		},
		WroteHeaders: func() {
			t.add("WroteHeaders", "")
		},
		WroteRequest: func(i httptrace.WroteRequestInfo) {
			t.add("WroteRequest", "%+v", i)
		},
		GotFirstResponseByte: func() {
			t.add("GotFirstResponseByte", "")
		},
		Got100Continue: func() {
			t.add("Got100Continue", "")
		},
		Wait100Continue: func() {
			t.add("Wait100Continue", "")
		},
		PutIdleConn: func(err error) {
			t.add("PutIdleConn", "err=%v", err)
		},
	})
}
