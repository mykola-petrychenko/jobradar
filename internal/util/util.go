package util

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var tagPattern = regexp.MustCompile(`<[^>]*>`)

var entityReplacer = strings.NewReplacer(
	"&#x26;", "&", "&amp;", "&", "&lt;", "<", "&gt;", ">",
	"&quot;", `"`, "&#39;", "'", "&nbsp;", " ",
)

// CleanHTML strips tags and decodes entities from posting description.
func CleanHTML(html string) string {
	text := tagPattern.ReplaceAllString(html, " ")
	text = entityReplacer.Replace(text)
	text = strings.ReplaceAll(text, `\n`, " ")
	text = strings.Join(strings.Fields(text), " ")
	return text
}

const debugDir = "debug"

// DumpRequest writes the outgoing API params as JSON for inspection.
func DumpRequest(name string, params any) {
	os.MkdirAll(debugDir, 0o755)
	data, err := json.MarshalIndent(params, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(filepath.Join(debugDir, name+"-request.json"), data, 0o644)
}

// DumpRawResponse writes the exact bytes the API returned.
func DumpRawResponse(name, rawJSON string) {
	os.MkdirAll(debugDir, 0o755)
	var pretty json.RawMessage = []byte(rawJSON)
	out, err := json.MarshalIndent(pretty, "", "  ")
	if err != nil {
		out = []byte(rawJSON)
	}
	os.WriteFile(filepath.Join(debugDir, name+"-response.json"), out, 0o644)
}
