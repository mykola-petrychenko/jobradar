package claude

import (
	"os"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// Client is the shared Anthropic API client for all jobradar calls.
type Client struct {
	api anthropic.Client
}

// NewClient reads ANTHROPIC_API_KEY from the environment.
func New() *Client {
	return &Client{
		api: anthropic.NewClient(option.WithAPIKey(os.Getenv("ANTHROPIC_API_KEY"))),
	}
}
