package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const debugDir = "debug"

func dumpRequest(name string, params any) {
	os.MkdirAll(debugDir, 0o755)
	data, err := json.MarshalIndent(params, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(filepath.Join(debugDir, name+"-request.json"), data, 0o644)
}

func dumpRawResponse(name, rawJSON string) {
	os.MkdirAll(debugDir, 0o755)
	var pretty json.RawMessage = []byte(rawJSON)
	out, err := json.MarshalIndent(pretty, "", "  ")
	if err != nil {
		out = []byte(rawJSON)
	}
	os.WriteFile(filepath.Join(debugDir, name+"-response.json"), out, 0o644)
}
