// The primary classifier: one question only — is_it true/false
package claude

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/mykola-petrychenko/jobradar/privat"
)

const ModelClassify = anthropic.ModelClaudeHaiku4_5

var classifySchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"is_it":       map[string]any{"type": "boolean"},
		"explanation": map[string]any{"type": "string"},
	},
	"required":             []string{"is_it", "explanation"},
	"additionalProperties": false,
}

// ClassifyResult is the verdict for one posting, plus call metadata
// needed for logging and token accounting.
type ClassifyResult struct {
	IsIT           bool
	Explanation    string
	Thinking       string
	Model          string
	InputTokens    int64
	OutputTokens   int64
	ThinkingTokens int64
}

func (c *Client) Classify(ctx context.Context, postingText string) (ClassifyResult, error) {
	params := anthropic.MessageNewParams{
		Model:     ModelClassify,
		MaxTokens: 1500,
		Thinking: anthropic.ThinkingConfigParamUnion{
			OfEnabled: &anthropic.ThinkingConfigEnabledParam{
				BudgetTokens: 1024,
			},
		},
		System: []anthropic.TextBlockParam{
			{Text: privat.ClassifyPrompt},
		},
		OutputConfig: anthropic.OutputConfigParam{
			Format: anthropic.JSONOutputFormatParam{Schema: classifySchema},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(postingText)),
		},
	}

	dumpRequest("classify", params)

	msg, err := c.api.Messages.New(ctx, params)
	if err != nil {
		return ClassifyResult{}, fmt.Errorf("classify: %w", err)
	}

	dumpRawResponse("classify", msg.RawJSON())

	if string(msg.StopReason) != "end_turn" {
		return ClassifyResult{}, fmt.Errorf("classify: unexpected stop_reason %q", msg.StopReason)
	}

	var rawText, thinkingText string
	for _, block := range msg.Content {
		switch block.Type {
		case "text":
			rawText = block.Text
		case "thinking":
			thinkingText = block.Thinking
		}
	}
	if rawText == "" {
		return ClassifyResult{}, fmt.Errorf("classify: no text block in response")
	}

	var parsed struct {
		IsIT        bool   `json:"is_it"`
		Explanation string `json:"explanation"`
	}
	if err := json.Unmarshal([]byte(rawText), &parsed); err != nil {
		return ClassifyResult{}, fmt.Errorf("classify: invalid JSON: %s", rawText)
	}

	return ClassifyResult{
		IsIT:           parsed.IsIT,
		Explanation:    parsed.Explanation,
		Thinking:       thinkingText,
		Model:          string(msg.Model),
		InputTokens:    msg.Usage.InputTokens,
		OutputTokens:   msg.Usage.OutputTokens,
		ThinkingTokens: msg.Usage.OutputTokensDetails.ThinkingTokens,
	}, nil
}
