// Package core holds the domain types shared by all jobradar components.
package core

// Posting is one raw job posting from any source.
type Posting struct {
	Source   string
	SourceID string
	Raw      []byte
}
