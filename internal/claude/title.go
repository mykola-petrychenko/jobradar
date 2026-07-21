// Title screening: a zero-cost pre-filter that sees only the posting
// title and rejects obviously non-IT jobs before the full classifier.
package claude

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/mykola-petrychenko/jobradar/privat"
)

// ModelTitleScreen runs the title-only pre-filter.
const ModelTitleScreen = anthropic.ModelClaudeHaiku4_5

// Verdict values the title screen can return.
const (
	VerdictIT     = "it"
	VerdictNotIT  = "not_it"
	VerdictUnsure = "unsure"
)

var titleScreenSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"verdict": map[string]any{
			"type": "string",
			"enum": []string{VerdictIT, VerdictNotIT, VerdictUnsure},
		},
	},
	"required":             []string{"verdict"},
	"additionalProperties": false,
}

// TitleScreenResult is the verdict for one title plus call metadata.
type TitleScreenResult struct {
	Verdict      string
	InputTokens  int64
	OutputTokens int64
	Model        string
}

// ScreenTitle classifies a posting by its title alone.
func (c *Client) ScreenTitle(ctx context.Context, title string) (TitleScreenResult, error) {
	msg, err := c.api.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     ModelTitleScreen,
		MaxTokens: 20,
		System: []anthropic.TextBlockParam{
			{Text: privat.TitleScreenPrompt},
		},
		OutputConfig: anthropic.OutputConfigParam{
			Format: anthropic.JSONOutputFormatParam{Schema: titleScreenSchema},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(title)),
		},
	})
	if err != nil {
		return TitleScreenResult{}, fmt.Errorf("title screen: %w", err)
	}
	if string(msg.StopReason) != "end_turn" {
		return TitleScreenResult{}, fmt.Errorf("title screen: unexpected stop_reason %q", msg.StopReason)
	}

	var rawText string
	for _, block := range msg.Content {
		if block.Type == "text" {
			rawText = block.Text
			break
		}
	}
	if rawText == "" {
		return TitleScreenResult{}, fmt.Errorf("title screen: no text block in response")
	}

	var parsed struct {
		Verdict string `json:"verdict"`
	}
	if err := json.Unmarshal([]byte(rawText), &parsed); err != nil {
		return TitleScreenResult{}, fmt.Errorf("title screen: invalid JSON: %s", rawText)
	}

	return TitleScreenResult{
		Verdict:      parsed.Verdict,
		InputTokens:  msg.Usage.InputTokens,
		OutputTokens: msg.Usage.OutputTokens,
		Model:        string(msg.Model),
	}, nil
}
