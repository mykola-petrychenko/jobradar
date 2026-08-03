// internal/arbeitnow/dump.go
package arbeitnow

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptrace"
	"net/http/httputil"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

var debugDir = func() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "debug")
}()

// connFacts holds what net/http itself reports as a request happens —
// not a reconstruction, the real negotiated outcome of each phase.
type connFacts struct {
	dnsStart, dnsDone         time.Time
	connectStart, connectDone time.Time
	tlsStart, tlsDone         time.Time
	negotiatedALPN            string // "h2" or "http/1.1" — the real ALPN result
	reusedConn                bool
	remoteAddr                string
}

// withConnTrace attaches hooks that fill facts as the real connection
// progresses. Nothing here is guessed or hardcoded for display.
func withConnTrace(ctx context.Context, facts *connFacts) context.Context {
	trace := &httptrace.ClientTrace{
		DNSStart: func(httptrace.DNSStartInfo) { facts.dnsStart = time.Now() },
		DNSDone:  func(httptrace.DNSDoneInfo) { facts.dnsDone = time.Now() },
		ConnectStart: func(network, addr string) {
			facts.connectStart = time.Now()
		},
		ConnectDone: func(network, addr string, err error) {
			facts.connectDone = time.Now()
			facts.remoteAddr = addr
		},
		TLSHandshakeStart: func() { facts.tlsStart = time.Now() },
		TLSHandshakeDone: func(state tls.ConnectionState, err error) {
			facts.tlsDone = time.Now()
			facts.negotiatedALPN = state.NegotiatedProtocol // the truth about h2 vs http/1.1
		},
		GotConn: func(info httptrace.GotConnInfo) {
			facts.reusedConn = info.Reused // true = existing TCP conn reused, no new handshake happened
		},
	}
	return httptrace.WithClientTrace(ctx, trace)
}

func dumpExchange(pageNum int, req *http.Request, resp *http.Response, facts connFacts, bodyBytes []byte) {
	os.MkdirAll(debugDir, 0o755)

	reqDump, _ := httputil.DumpRequestOut(req, true)
	respDump, _ := httputil.DumpResponse(resp, false) // headers only — body dumped separately below

	var out bytes.Buffer
	out.WriteString("=== REQUEST (raw wire format) ===\n")
	out.Write(reqDump)

	out.WriteString("\n\n=== RESPONSE (raw wire format, headers only) ===\n")
	out.Write(respDump)

	out.WriteString("\n\n=== REAL CONNECTION FACTS (not display text — actual negotiated outcome) ===\n")
	fmt.Fprintf(&out, "negotiated_alpn_protocol: %s\n", facts.negotiatedALPN)
	fmt.Fprintf(&out, "reused_existing_connection: %v\n", facts.reusedConn)
	fmt.Fprintf(&out, "remote_addr: %s\n", facts.remoteAddr)
	if !facts.dnsStart.IsZero() {
		fmt.Fprintf(&out, "dns_lookup_ms: %d\n", facts.dnsDone.Sub(facts.dnsStart).Milliseconds())
	}
	if !facts.connectStart.IsZero() {
		fmt.Fprintf(&out, "tcp_connect_ms: %d\n", facts.connectDone.Sub(facts.connectStart).Milliseconds())
	}
	if !facts.tlsStart.IsZero() {
		fmt.Fprintf(&out, "tls_handshake_ms: %d\n", facts.tlsDone.Sub(facts.tlsStart).Milliseconds())
	}

	stamp := time.Now().Format("150405.000")

	httpName := fmt.Sprintf("page-%03d-%s.http", pageNum, stamp)
	os.WriteFile(filepath.Join(debugDir, httpName), out.Bytes(), 0o644)

	var pretty bytes.Buffer
	if err := json.Indent(&pretty, bodyBytes, "", "  "); err != nil {
		pretty.Write(bodyBytes)
	}
	jsonName := fmt.Sprintf("page-%03d-%s.json", pageNum, stamp)
	os.WriteFile(filepath.Join(debugDir, jsonName), pretty.Bytes(), 0o644)
}
