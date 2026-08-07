// Package job defines the domain types shared by all jobradar components.
package job

import "encoding/json"

// Posting is a single raw job posting from any source.
type Posting struct {
	Source   string          // source name, e.g. "arbeitnow"
	SourceID string          // posting identifier within the source
	Raw      json.RawMessage // full posting JSON exactly as the source returned it
}
