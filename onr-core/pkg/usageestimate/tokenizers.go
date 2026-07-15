package usageestimate

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"

	tiktoken "github.com/pkoukk/tiktoken-go"
)

const (
	textCounterTiktoken = "tiktoken"
	textCounterTextLen  = "text_len"
)

type Tokenizer interface {
	CountToken(string) int
	ApplyChatTemplate() string
}

func GetTokenizers(ectx *EstimateContext) (Tokenizer, error) {
	switch {
	// case ectx.Model == "glm-5.2":
	// 	return NewOpenSourceTokenizer(ectx)
	case strings.HasPrefix(ectx.Model, "claude"):
		return NewCloseSourceTokenizer(ectx)
	}
	return NewCloseSourceTokenizer(ectx)
}

type CloseSourceTokenizer struct {
	ectx        *EstimateContext
	profile     ClosedSourceProfile
	enc         *tiktoken.Tiktoken
	textBuilder *strings.Builder
}

func NewCloseSourceTokenizer(ectx *EstimateContext) (*CloseSourceTokenizer, error) {
	profile := closedSourceProfileForAPIModel(ectx.API, ectx.Model)
	return &CloseSourceTokenizer{
		ectx:    ectx,
		profile: profile,
		enc:     ectx.TokenEncoder,
	}, nil
}

func (ct *CloseSourceTokenizer) aggregateText(text string) {
	if text == "" {
		return
	}
	ct.textBuilder.Write([]byte(text + " "))

}
func (ct *CloseSourceTokenizer) CountToken(prompt string) int {
	//count overhead token
	overheadTokens := 0.0
	hiddenReasonInOutput := false
	for item, num := range ct.ectx.OverHeadItems {
		if ct.ectx.Direction == EstimateOutput && item == ItemHiddenReasoningBlock {
			hiddenReasonInOutput = true
			continue
		}
		weight, ok := ct.profile.OverheadWeightOverrides[item]
		if !ok {
			weight = float64(ct.profile.OverheadWeights[item])
		}
		overheadTokens += weight * float64(num)
	}
	textTokens, textCounter := ct.countTextTokens(prompt)
	textMultiplier := ct.profile.textMultiplierForCounterDirection(textCounter, textTokens, ct.ectx.Direction)
	scaledTextTokens := int(math.Round(float64(textTokens) * textMultiplier))
	sumTokens := scaledTextTokens + int(math.Round(overheadTokens))
	if hiddenReasonInOutput {
		sumTokens = int(math.Round(float64(sumTokens) * ct.profile.hiddenReasoningOutputMultiplier()))
	}
	return sumTokens
}

func (ct *CloseSourceTokenizer) countTextTokens(text string) (int, string) {
	switch normalizeTextCounter(ct.profile.TextCounter) {
	case textCounterTiktoken:
		if ct.enc != nil {
			return len(ct.enc.EncodeOrdinary(text)), textCounterTiktoken
		}
		return countTextLenTokens(text), textCounterTextLen
	case textCounterTextLen:
		return countTextLenTokens(text), textCounterTextLen
	default:
		return countTextLenTokens(text), textCounterTextLen
	}
}

// countTextLenTokens estimates token count from text length when no real
// tokenizer is available. Rather than dividing raw byte length by a single
// factor (which distorts badly for CJK, where one character is ~3 bytes but is
// token-dense), it walks the text once by rune, classifies each character, and
// applies a per-class factor. The factors are rough and meant to be corrected
// downstream by the profile's text-length multiplier bands.
func countTextLenTokens(text string) int {
	var latin, cjk, other int
	for _, r := range text {
		switch {
		case r < 0x80:
			// ASCII / Latin: ~4 characters per token.
			latin++
		case unicode.Is(unicode.Han, r),
			unicode.Is(unicode.Hiragana, r),
			unicode.Is(unicode.Katakana, r),
			unicode.Is(unicode.Hangul, r):
			// CJK: token-dense, ~1.5 tokens per character.
			cjk++
		default:
			// Other scripts / punctuation / emoji: ~3 characters per token.
			other++
		}
	}
	return latin/4 + cjk*3/2 + other/3
}

func (ct *CloseSourceTokenizer) aggregateMessagesRequestText() {

	for _, message := range ct.ectx.Messages {
		ct.aggregateText(message.Name)
		for _, messageItem := range message.Content {
			switch messageItem.Type {
			case "text", "thinking", "refusal":
				ct.aggregateText(messageItem.Text)
			case "tool_use", "server_tool_use":
				ct.aggregateText(messageItem.Name)
				ct.aggregateArgumentText(messageItem.Arguments)
			case "tool_result", "web_search_tool_result":
				ct.aggregateText(messageItem.Text)
				//todo content aggregate

			}

		}
		for _, toolCall := range message.ToolCalls {
			ct.aggregateText(toolCall.Name)
			ct.aggregateArgumentText(toolCall.Arguments)
		}

	}
	for _, text := range ct.ectx.Texts {
		ct.aggregateText(text.Text)
	}

	for _, tool := range ct.ectx.Tools {
		ct.aggregateText(tool.Name)
		ct.aggregateText(tool.Description)
		ct.aggregateText(tool.Definition)
		ct.aggregateToolSchemaText(&tool.Parameters)

	}

}

func (ct *CloseSourceTokenizer) aggregateArgumentText(value any) {
	switch v := value.(type) {
	case nil:
		return
	case string:
		ct.aggregateText(v)
	case bool:
		ct.aggregateText(fmt.Sprintf("%t", v))
	case float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		ct.aggregateText(fmt.Sprintf("%v", v))
	case []any:
		for _, item := range v {
			ct.aggregateArgumentText(item)
		}
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			ct.aggregateText(key)
			ct.aggregateArgumentText(v[key])
		}
	default:
		ct.aggregateText(fmt.Sprintf("%v", v))
	}
}

func (ct *CloseSourceTokenizer) aggregateToolSchemaText(schema *ToolSchema) {
	if schema == nil {
		return
	}
	if schema.Description != "" {
		ct.aggregateText(schema.Description)
	}

	if len(schema.Required) != 0 {
		for _, text := range schema.Required {
			ct.aggregateText(text)
		}
	}

	if len(schema.Enum) != 0 {
		for _, ob := range schema.Enum {
			if text, ok := ob.(string); ok {
				ct.aggregateText(text)
			}

		}
	}

	if schema.Items != nil {
		ct.aggregateToolSchemaText(schema.Items)
	}

	for name, v := range schema.Properties {
		ct.aggregateText(name)
		ct.aggregateToolSchemaText(v)

	}
	for _, v := range schema.AnyOf {
		ct.aggregateToolSchemaText(v)
	}
	for _, v := range schema.OneOf {
		ct.aggregateToolSchemaText(v)
	}
	for _, v := range schema.AllOf {
		ct.aggregateToolSchemaText(v)
	}

}

func (ct *CloseSourceTokenizer) ApplyChatTemplate() string {
	ct.textBuilder = &strings.Builder{}

	switch ct.ectx.Direction {
	case EstimateInput:
		switch normalizeAPI(ct.ectx.API) {
		case apiMessages, apiChatCompletions, apiResponses, apiGeminiGenerateContent, apiGeminiStreamGenerateContent:
			ct.aggregateMessagesRequestText()
		}
	case EstimateOutput:
		switch normalizeAPI(ct.ectx.API) {
		case apiMessages, apiChatCompletions, apiResponses, apiGeminiGenerateContent, apiGeminiStreamGenerateContent:
			ct.aggregateMessagesRequestText()
		}
	}
	return strings.TrimSuffix(ct.textBuilder.String(), " ")
}

type OpenSourceTokenizer struct {
	ectx *EstimateContext
}

func NewOpenSourceTokenizer(ectx *EstimateContext) *OpenSourceTokenizer {

	return &OpenSourceTokenizer{ectx: ectx}
}
func (ot *OpenSourceTokenizer) CountToken(prompt string) int {
	return 0
}
func (ot *OpenSourceTokenizer) CountTokenByText(prompt string) int {
	return 0
}
func (ot *OpenSourceTokenizer) ApplyChatTemplate() string {
	return ""
}
