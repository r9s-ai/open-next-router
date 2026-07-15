package usageestimate

import (
	"strings"

	tiktoken "github.com/pkoukk/tiktoken-go"
)

type EstimateDirection string
type OverHeadItemKind string

const (
	ItemPromptBase OverHeadItemKind = "prompt_base"

	ItemMessage       OverHeadItemKind = "message"
	ItemSystemMessage OverHeadItemKind = "system_message"
	ItemRoleUser      OverHeadItemKind = "user"
	ItemRoleAssistant OverHeadItemKind = "assistant"
	ItemRoleTool      OverHeadItemKind = "tool"
	ItemRoleSystem    OverHeadItemKind = "developer"

	ItemToolSection     OverHeadItemKind = "tool_section"
	ItemToolDefinition  OverHeadItemKind = "tool_definition"
	ItemToolDescription OverHeadItemKind = "tool_description"

	ItemFunctionCall       OverHeadItemKind = "function_call"
	ItemFunctionCallResult OverHeadItemKind = "function_call_output"

	ItemResponseFormatJsonSchema                       OverHeadItemKind = "response_format_json_schema"
	ItemResponseFormatJsonSchemaStringPropertyRequired OverHeadItemKind = "response_format_json_schema_string_property_required"

	ItemCustomToolCall       OverHeadItemKind = "custom_tool_call"
	ItemCustomToolCallOutput OverHeadItemKind = "custom_tool_call_output"

	ItemHiddenReasoningBlock OverHeadItemKind = "hidden_reasoning_block"
	ItemThinkingBlock        OverHeadItemKind = "thinking_block"
	ItemThinkingSignature    OverHeadItemKind = "thinking_signature"
	ItemImageBlock           OverHeadItemKind = "image_black"
	ItemDocumentBlock        OverHeadItemKind = "document_black"
	ItemWebSearch            OverHeadItemKind = "web_search"
	ItemUnknownBlcok         OverHeadItemKind = "unknown"
	//anthropic tool define
	ItemToolPropertiesTypeString        OverHeadItemKind = "tool_properties_type_string"
	ItemToolPropertiesTypeNumber        OverHeadItemKind = "tool_properties_type_number"
	ItemToolPropertiesTypeInt           OverHeadItemKind = "tool_properties_type_int"
	ItemToolPropertiesTypeBool          OverHeadItemKind = "tool_properties_type_bool"
	ItemToolPropertiesTypeArray         OverHeadItemKind = "tool_properties_type_array"
	ItemToolPropertiesTypeArrayOfString OverHeadItemKind = "tool_properties_type_array_of_string"
	ItemToolPropertiesTypeObject        OverHeadItemKind = "tool_properties_type_object"

	ItemToolPropertiesDescription           OverHeadItemKind = "tool_properties_description"
	ItemToolProperties                      OverHeadItemKind = "tool_properties"
	ItemToolPropertiesItem                  OverHeadItemKind = "tool_properties_item"
	ItemToolRequired                        OverHeadItemKind = "tool_required"
	ItemToolRequiredItem                    OverHeadItemKind = "tool_required_item"
	ItemToolEnum                            OverHeadItemKind = "tool_enum"
	ItemToolEnumItem                        OverHeadItemKind = "tool_enum_item"
	ItemTooladditionalProperties            OverHeadItemKind = "tool_additionalProperties"
	ItemTooladditionalPropertiesBool        OverHeadItemKind = "tool_additionalProperties_false"
	ItemTooladditionalPropertiesTypeString  OverHeadItemKind = "tool_additionalProperties_type_string"
	ItemTooladditionalPropertiesTypeUnknown OverHeadItemKind = "tool_additionalProperties_type_unknown"
	//
	ItemToolChoiceAny      OverHeadItemKind = "tool_choice_any"
	ItemToolChoiceNone     OverHeadItemKind = "tool_choice_none"
	ItemToolChoiceAuto     OverHeadItemKind = "tool_choice_auto"
	ItemToolChoiceToolName OverHeadItemKind = "tool_choice_tool_name"
	//anthropic tool use

	ItemToolUseBlockInput OverHeadItemKind = "tool_use_input"

	ItemToolUseBlockInputItem   OverHeadItemKind = "tool_use_input_item"
	ItemToolUseBlockInputString OverHeadItemKind = "tool_use_input_string"
	ItemToolUseBlockInputInt    OverHeadItemKind = "tool_use_input_int"
	ItemToolUseBlockInputBool   OverHeadItemKind = "tool_use_input_bool"
	ItemToolUseBlockInputList   OverHeadItemKind = "tool_use_input_list"
)
const (
	EstimateInput  = "estimate_input"
	EstimateOutput = "estimate_output"
)

type EstimateContext struct {
	API               string
	Model             string
	IsStream          bool
	MaxTokens         int
	MaxThinkingTokens int
	Direction         EstimateDirection
	// TokenEncoder is optional. When non-nil, callers have already initialized
	// the encoder and NewCloseSourceTokenizer reuses it. When nil, token
	// counting falls back to the profile's text-length counter.
	TokenEncoder *tiktoken.Tiktoken

	Messages      []EstimateMessage
	ToolChoice    map[string]string
	Tools         []EstimateTool
	Texts         []EstimateText
	OverHeadItems map[OverHeadItemKind]int
}

func NewEstimateContext(modelName, api string, estimateDirection EstimateDirection) *EstimateContext {
	name := strings.TrimSpace(strings.ToLower(modelName))
	c := &EstimateContext{API: api, Model: name, Direction: estimateDirection}
	return c

}
func (ectx *EstimateContext) AddOverHead(kind OverHeadItemKind, num int) {
	if ectx.OverHeadItems == nil {
		ectx.OverHeadItems = make(map[OverHeadItemKind]int)
	}

	ectx.OverHeadItems[kind] += num

}

type EstimateMessage struct {
	Role string
	Name string

	Content    []EstimateMessagesContent
	ToolCalls  []EstimateToolCall // OpenAI message-level, plus optional normalized index
	ToolCallID string
}

type EstimateMessagesContent struct {
	Type      string
	Text      string //text or tool call content
	Signature string
	Raw       any
	// Anthropic/Gemini content-level calls
	ID        string
	Name      string
	Arguments map[string]any
	Content   []EstimateMessagesContent
}

type EstimateTool struct {
	Type        string
	Name        string
	Description string
	Definition  string
	Parameters  ToolSchema
	Raw         any
}

type ToolSchema struct {
	Type                 any                    `json:"type,omitempty"`
	Description          string                 `json:"description,omitempty"`
	Properties           map[string]*ToolSchema `json:"properties,omitempty"`
	Required             []string               `json:"required,omitempty"`
	Items                *ToolSchema            `json:"items,omitempty"`
	AdditionalProperties any                    `json:"additionalProperties,omitempty"`
	Enum                 []any                  `json:"enum,omitempty"`
	AnyOf                []*ToolSchema          `json:"anyOf,omitempty"`
	OneOf                []*ToolSchema          `json:"oneOf,omitempty"`
	AllOf                []*ToolSchema          `json:"allOf,omitempty"`
	Format               string                 `json:"format,omitempty"`
	Pattern              string                 `json:"pattern,omitempty"`
}

type EstimateToolCall struct {
	Type      string
	ID        string
	Name      string
	Arguments any
	Raw       any
}

type EstimateText struct {
	Kind string
	Text string
}
