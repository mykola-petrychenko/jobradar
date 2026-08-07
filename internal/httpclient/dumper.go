package httpclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"os"
	"path/filepath"
	"time"
)

// Dumper records the raw wire exchange for one request.
type Dumper interface {
	Dump(label string, req *http.Request, resp *http.Response, trace *connTrace, body []byte)
}

type noopDumper struct{}

func (noopDumper) Dump(string, *http.Request, *http.Response, *connTrace, []byte) {}

type fileDumper struct {
	dir    string
	logger *slog.Logger
}

func (d fileDumper) Dump(label string, req *http.Request, resp *http.Response, trace *connTrace, body []byte) {
	if err := os.MkdirAll(d.dir, 0o755); err != nil {
		d.logger.Warn("dumper: create dir failed", "dir", d.dir, "err", err)
		return
	}

	stamp := time.Now().Format("150405.000")
	d.write(fmt.Sprintf("%s-%s.http", label, stamp), formatExchange(req, resp, trace))
	d.write(fmt.Sprintf("%s-%s.json", label, stamp), prettyJSON(body))
}

func (d fileDumper) write(name string, content []byte) {
	path := filepath.Join(d.dir, name)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		d.logger.Warn("dumper: write failed", "path", path, "err", err)
	}
}

func formatExchange(req *http.Request, resp *http.Response, trace *connTrace) []byte {
	reqDump, _ := httputil.DumpRequestOut(req, true)
	respDump, _ := httputil.DumpResponse(resp, false)

	var out bytes.Buffer
	out.WriteString("=== REQUEST ===\n")
	out.Write(reqDump)
	out.WriteString("\n=== RESPONSE HEADERS ===\n")
	out.Write(respDump)
	out.WriteString("\n=== TRACE ===\n")
	out.WriteString(trace.String())
	return out.Bytes()
}

func prettyJSON(body []byte) []byte {
	var out bytes.Buffer
	if err := json.Indent(&out, body, "", "  "); err != nil {
		return body
	}
	return out.Bytes()
}
