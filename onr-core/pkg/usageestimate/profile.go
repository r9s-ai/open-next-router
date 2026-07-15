package usageestimate

import "strings"

func closedSourceProfileForAPIModel(api, model string) ClosedSourceProfile {
	a := normalizeAPI(api)
	m := strings.ToLower(strings.TrimSpace(model))

	if profile, ok := findClosedSourceProfile(a, m); ok {
		return profile
	}
	if profile, ok := fallbackClosedSourceProfile(a, m); ok {
		return profile
	}
	return ClosedSourceProfile{TextMultiplier: 1, OverheadWeights: map[OverHeadItemKind]int{}}
}

func fallbackClosedSourceProfile(api, model string) (ClosedSourceProfile, bool) {
	switch api {
	case apiResponses:
		return findClosedSourceProfile(apiResponses, "gpt-5.5")
	case apiChatCompletions:
		return findClosedSourceProfile(apiChatCompletions, "gpt-5.5")
	case apiMessages:
		return findClosedSourceProfile(apiMessages, "claude-opus-4-8")
	case apiGeminiGenerateContent:
		return findClosedSourceProfile(apiGeminiGenerateContent, fallbackGeminiModel(model))
	case apiGeminiStreamGenerateContent:
		return findClosedSourceProfile(apiGeminiStreamGenerateContent, fallbackGeminiModel(model))
	default:
		return ClosedSourceProfile{}, false
	}
}

func fallbackGeminiModel(model string) string {
	if strings.Contains(model, "gemini-3") {
		return "gemini-3-flash-preview"
	}
	return "gemini-2.5-pro"
}

func findClosedSourceProfile(api, model string) (ClosedSourceProfile, bool) {
	for _, profile := range closedSourceProfiles {
		if normalizeAPI(profile.API) != api {
			continue
		}
		if profile.matchesModel(model) {
			return profile, true
		}
	}
	return ClosedSourceProfile{}, false
}

type ClosedSourceProfile struct {
	API                                string
	ModelContains                      []string
	TextCounter                        string
	TextMultiplier                     float64
	TextMultiplierBands                []TextMultiplierBand
	TextMultiplierBandsByCounter       map[string][]TextMultiplierBand
	OutputTextMultiplier               float64
	OutputTextMultiplierBands          []TextMultiplierBand
	OutputTextMultiplierBandsByCounter map[string][]TextMultiplierBand
	HiddenReasoningOutputMultiplier    float64
	BasePad                            int
	OverheadWeights                    map[OverHeadItemKind]int
	OverheadWeightOverrides            map[OverHeadItemKind]float64
}

func (p ClosedSourceProfile) matchesModel(model string) bool {
	for _, fragment := range p.ModelContains {
		if strings.Contains(model, strings.ToLower(strings.TrimSpace(fragment))) {
			return true
		}
	}
	return false
}

type TextMultiplierBand struct {
	MaxTokens  int
	Multiplier float64
}

func (p ClosedSourceProfile) textMultiplierFor(textTokens int) float64 {
	return p.textMultiplierForCounter(p.TextCounter, textTokens)
}

func (p ClosedSourceProfile) textMultiplierForCounter(counter string, textTokens int) float64 {
	counter = normalizeTextCounter(counter)
	if bands := p.TextMultiplierBandsByCounter[counter]; len(bands) != 0 {
		return textMultiplierForBands(bands, textTokens, p.TextMultiplier)
	}
	return textMultiplierForBands(p.TextMultiplierBands, textTokens, p.TextMultiplier)
}

func (p ClosedSourceProfile) textMultiplierForCounterDirection(counter string, textTokens int, direction EstimateDirection) float64 {
	if direction != EstimateOutput {
		return p.textMultiplierForCounter(counter, textTokens)
	}
	counter = normalizeTextCounter(counter)
	if bands := p.OutputTextMultiplierBandsByCounter[counter]; len(bands) != 0 {
		return textMultiplierForBands(bands, textTokens, p.OutputTextMultiplier)
	}
	if len(p.OutputTextMultiplierBands) != 0 || p.OutputTextMultiplier > 0 {
		return textMultiplierForBands(p.OutputTextMultiplierBands, textTokens, p.OutputTextMultiplier)
	}
	return p.textMultiplierForCounter(counter, textTokens)
}

func (p ClosedSourceProfile) hiddenReasoningOutputMultiplier() float64 {
	if p.HiddenReasoningOutputMultiplier > 0 {
		return p.HiddenReasoningOutputMultiplier
	}
	return 1
}

func textMultiplierForBands(bands []TextMultiplierBand, textTokens int, fallback float64) float64 {
	for _, band := range bands {
		if band.Multiplier <= 0 {
			continue
		}
		if band.MaxTokens <= 0 || textTokens <= band.MaxTokens {
			return band.Multiplier
		}
	}
	if fallback > 0 {
		return fallback
	}
	return 1
}

func normalizeTextCounter(counter string) string {
	switch strings.ToLower(strings.TrimSpace(counter)) {
	case "", textCounterTiktoken:
		return textCounterTiktoken
	case textCounterTextLen:
		return textCounterTextLen
	default:
		return textCounterTextLen
	}
}

var closedSourceProfiles = []ClosedSourceProfile{
	{
		API:                             apiChatCompletions,
		ModelContains:                   []string{"gpt-5.5"},
		TextCounter:                     "tiktoken",
		TextMultiplier:                  1,
		HiddenReasoningOutputMultiplier: 1.3,
		TextMultiplierBands: []TextMultiplierBand{
			{MaxTokens: 1024 * 10, Multiplier: 1},
			{MaxTokens: 1024 * 20, Multiplier: 1},
			{MaxTokens: 1024 * 30, Multiplier: 1},
			{MaxTokens: 1024 * 40, Multiplier: 1},
			{MaxTokens: 1024 * 50, Multiplier: 1},
			{MaxTokens: 1024 * 60, Multiplier: 1},
			{MaxTokens: 1024 * 70, Multiplier: 1},
			{MaxTokens: 1024 * 80, Multiplier: 1},
			{MaxTokens: 1024 * 90, Multiplier: 1},
			{MaxTokens: 1024 * 100, Multiplier: 1},
			{MaxTokens: 0, Multiplier: 1},
		},
		TextMultiplierBandsByCounter: map[string][]TextMultiplierBand{
			textCounterTiktoken: []TextMultiplierBand{
				{MaxTokens: 1024 * 10, Multiplier: 1},
				{MaxTokens: 1024 * 20, Multiplier: 1},
				{MaxTokens: 1024 * 30, Multiplier: 1},
				{MaxTokens: 1024 * 40, Multiplier: 1},
				{MaxTokens: 1024 * 50, Multiplier: 1},
				{MaxTokens: 1024 * 60, Multiplier: 1},
				{MaxTokens: 1024 * 70, Multiplier: 1},
				{MaxTokens: 1024 * 80, Multiplier: 1},
				{MaxTokens: 1024 * 90, Multiplier: 1},
				{MaxTokens: 1024 * 100, Multiplier: 1},
				{MaxTokens: 0, Multiplier: 1},
			},
			textCounterTextLen: []TextMultiplierBand{
				{MaxTokens: 1024 * 10, Multiplier: 1},
				{MaxTokens: 1024 * 20, Multiplier: 1},
				{MaxTokens: 1024 * 30, Multiplier: 1},
				{MaxTokens: 1024 * 40, Multiplier: 1},
				{MaxTokens: 1024 * 50, Multiplier: 1},
				{MaxTokens: 1024 * 60, Multiplier: 1},
				{MaxTokens: 1024 * 70, Multiplier: 1},
				{MaxTokens: 1024 * 80, Multiplier: 1},
				{MaxTokens: 1024 * 90, Multiplier: 1},
				{MaxTokens: 1024 * 100, Multiplier: 1},
				{MaxTokens: 0, Multiplier: 1},
			},
		},
		OverheadWeights: map[OverHeadItemKind]int{
			ItemPromptBase:           2,
			ItemRoleUser:             4,
			ItemRoleAssistant:        6,
			ItemRoleSystem:           4,
			ItemToolSection:          97,
			ItemToolDefinition:       6,
			ItemToolDescription:      2,
			ItemFunctionCall:         7,
			ItemFunctionCallResult:   7,
			ItemHiddenReasoningBlock: 300,

			ItemToolChoiceAny:      0,
			ItemToolChoiceNone:     0,
			ItemToolChoiceAuto:     0,
			ItemToolChoiceToolName: 0,

			ItemToolPropertiesTypeString:        5,
			ItemToolPropertiesTypeNumber:        5,
			ItemToolPropertiesTypeInt:           5,
			ItemToolPropertiesTypeBool:          5,
			ItemToolPropertiesTypeObject:        0,
			ItemToolPropertiesTypeArray:         0,
			ItemToolPropertiesTypeArrayOfString: 1,

			ItemToolPropertiesDescription:    2,
			ItemToolProperties:               0,
			ItemToolPropertiesItem:           0,
			ItemToolRequired:                 0,
			ItemToolRequiredItem:             0,
			ItemToolEnum:                     2,
			ItemToolEnumItem:                 1,
			ItemTooladditionalProperties:     0,
			ItemTooladditionalPropertiesBool: 0,
		},
	},
	{
		API:                             apiResponses,
		ModelContains:                   []string{"gpt-5.5"},
		TextCounter:                     "tiktoken",
		TextMultiplier:                  1,
		HiddenReasoningOutputMultiplier: 1.3,
		TextMultiplierBands: []TextMultiplierBand{
			{MaxTokens: 1024 * 10, Multiplier: 1},
			{MaxTokens: 1024 * 20, Multiplier: 1},
			{MaxTokens: 1024 * 30, Multiplier: 1},
			{MaxTokens: 1024 * 40, Multiplier: 1},
			{MaxTokens: 1024 * 50, Multiplier: 1},
			{MaxTokens: 1024 * 60, Multiplier: 1},
			{MaxTokens: 1024 * 70, Multiplier: 1},
			{MaxTokens: 1024 * 80, Multiplier: 1},
			{MaxTokens: 1024 * 90, Multiplier: 1},
			{MaxTokens: 1024 * 100, Multiplier: 1},
			{MaxTokens: 0, Multiplier: 1},
		},
		TextMultiplierBandsByCounter: map[string][]TextMultiplierBand{
			textCounterTiktoken: []TextMultiplierBand{
				{MaxTokens: 1024 * 10, Multiplier: 1},
				{MaxTokens: 1024 * 20, Multiplier: 1},
				{MaxTokens: 1024 * 30, Multiplier: 1},
				{MaxTokens: 1024 * 40, Multiplier: 1},
				{MaxTokens: 1024 * 50, Multiplier: 1},
				{MaxTokens: 1024 * 60, Multiplier: 1},
				{MaxTokens: 1024 * 70, Multiplier: 1},
				{MaxTokens: 1024 * 80, Multiplier: 1},
				{MaxTokens: 1024 * 90, Multiplier: 1},
				{MaxTokens: 1024 * 100, Multiplier: 1},
				{MaxTokens: 0, Multiplier: 1},
			},
			textCounterTextLen: []TextMultiplierBand{
				{MaxTokens: 1024 * 10, Multiplier: 1},
				{MaxTokens: 1024 * 20, Multiplier: 1},
				{MaxTokens: 1024 * 30, Multiplier: 1},
				{MaxTokens: 1024 * 40, Multiplier: 1},
				{MaxTokens: 1024 * 50, Multiplier: 1},
				{MaxTokens: 1024 * 60, Multiplier: 1},
				{MaxTokens: 1024 * 70, Multiplier: 1},
				{MaxTokens: 1024 * 80, Multiplier: 1},
				{MaxTokens: 1024 * 90, Multiplier: 1},
				{MaxTokens: 1024 * 100, Multiplier: 1},
				{MaxTokens: 0, Multiplier: 1},
			},
		},
		OverheadWeights: map[OverHeadItemKind]int{
			ItemPromptBase:               2,
			ItemRoleUser:                 4,
			ItemRoleAssistant:            6,
			ItemRoleSystem:               4,
			ItemRoleTool:                 4,
			ItemToolSection:              15,
			ItemToolDefinition:           6,
			ItemToolDescription:          2,
			ItemFunctionCall:             5,
			ItemFunctionCallResult:       6,
			ItemResponseFormatJsonSchema: 16,
			ItemResponseFormatJsonSchemaStringPropertyRequired: 5,
			ItemHiddenReasoningBlock:                           300,

			ItemToolChoiceAny:      0,
			ItemToolChoiceNone:     0,
			ItemToolChoiceAuto:     0,
			ItemToolChoiceToolName: 0,

			ItemToolPropertiesTypeString:        4,
			ItemToolPropertiesTypeNumber:        5,
			ItemToolPropertiesTypeInt:           5,
			ItemToolPropertiesTypeBool:          5,
			ItemToolPropertiesTypeObject:        0,
			ItemToolPropertiesTypeArray:         0,
			ItemToolPropertiesTypeArrayOfString: 1,

			ItemToolPropertiesDescription:    2,
			ItemToolProperties:               0,
			ItemToolPropertiesItem:           0,
			ItemToolRequired:                 0,
			ItemToolRequiredItem:             0,
			ItemToolEnum:                     2,
			ItemToolEnumItem:                 1,
			ItemTooladditionalProperties:     0,
			ItemTooladditionalPropertiesBool: 0,
		},
	},
	{
		API:                             apiGeminiGenerateContent,
		ModelContains:                   []string{"gemini-2.5-pro"},
		TextCounter:                     "tiktoken",
		TextMultiplier:                  1,
		HiddenReasoningOutputMultiplier: 1.3,
		TextMultiplierBands: []TextMultiplierBand{
			{MaxTokens: 1024 * 10, Multiplier: 1},
			{MaxTokens: 1024 * 20, Multiplier: 1},
			{MaxTokens: 1024 * 30, Multiplier: 1},
			{MaxTokens: 1024 * 40, Multiplier: 1},
			{MaxTokens: 1024 * 50, Multiplier: 1},
			{MaxTokens: 1024 * 60, Multiplier: 1},
			{MaxTokens: 1024 * 70, Multiplier: 1},
			{MaxTokens: 1024 * 80, Multiplier: 1},
			{MaxTokens: 1024 * 90, Multiplier: 1},
			{MaxTokens: 1024 * 100, Multiplier: 1},
			{MaxTokens: 0, Multiplier: 1},
		},
		TextMultiplierBandsByCounter: map[string][]TextMultiplierBand{
			textCounterTiktoken: []TextMultiplierBand{
				{MaxTokens: 1024 * 10, Multiplier: 1},
				{MaxTokens: 1024 * 20, Multiplier: 1},
				{MaxTokens: 1024 * 30, Multiplier: 1},
				{MaxTokens: 1024 * 40, Multiplier: 1},
				{MaxTokens: 1024 * 50, Multiplier: 1},
				{MaxTokens: 1024 * 60, Multiplier: 1},
				{MaxTokens: 1024 * 70, Multiplier: 1},
				{MaxTokens: 1024 * 80, Multiplier: 1},
				{MaxTokens: 1024 * 90, Multiplier: 1},
				{MaxTokens: 1024 * 100, Multiplier: 1},
				{MaxTokens: 0, Multiplier: 1},
			},
			textCounterTextLen: []TextMultiplierBand{
				{MaxTokens: 1024 * 10, Multiplier: 1},
				{MaxTokens: 1024 * 20, Multiplier: 1},
				{MaxTokens: 1024 * 30, Multiplier: 1},
				{MaxTokens: 1024 * 40, Multiplier: 1},
				{MaxTokens: 1024 * 50, Multiplier: 1},
				{MaxTokens: 1024 * 60, Multiplier: 1},
				{MaxTokens: 1024 * 70, Multiplier: 1},
				{MaxTokens: 1024 * 80, Multiplier: 1},
				{MaxTokens: 1024 * 90, Multiplier: 1},
				{MaxTokens: 1024 * 100, Multiplier: 1},
				{MaxTokens: 0, Multiplier: 1},
			},
		},
		OverheadWeights: map[OverHeadItemKind]int{
			ItemPromptBase:         0,
			ItemRoleUser:           0,
			ItemRoleAssistant:      0,
			ItemRoleSystem:         0,
			ItemRoleTool:           0,
			ItemToolSection:        0,
			ItemToolDefinition:     0,
			ItemToolDescription:    0,
			ItemFunctionCall:       0,
			ItemFunctionCallResult: 0,

			ItemResponseFormatJsonSchema: 0,
			ItemThinkingBlock:            0,
			ItemHiddenReasoningBlock:     0,
			ItemImageBlock:               0,
			ItemDocumentBlock:            0,
			ItemUnknownBlcok:             0,

			ItemToolPropertiesTypeString:        1,
			ItemToolPropertiesTypeNumber:        1,
			ItemToolPropertiesTypeInt:           1,
			ItemToolPropertiesTypeBool:          1,
			ItemToolPropertiesTypeObject:        1,
			ItemToolPropertiesTypeArray:         0,
			ItemToolPropertiesTypeArrayOfString: 1,

			ItemToolPropertiesDescription:           0,
			ItemToolRequired:                        0,
			ItemToolRequiredItem:                    0,
			ItemToolEnum:                            0,
			ItemToolEnumItem:                        1,
			ItemTooladditionalPropertiesBool:        0,
			ItemTooladditionalPropertiesTypeString:  0,
			ItemTooladditionalPropertiesTypeUnknown: 0,
		},
	},
	{
		API:                             apiGeminiGenerateContent,
		ModelContains:                   []string{"gemini-3-flash-preview"},
		TextCounter:                     "tiktoken",
		TextMultiplier:                  1,
		HiddenReasoningOutputMultiplier: 1.3,
		TextMultiplierBands: []TextMultiplierBand{
			{MaxTokens: 1024 * 10, Multiplier: 1},
			{MaxTokens: 1024 * 20, Multiplier: 1},
			{MaxTokens: 1024 * 30, Multiplier: 1},
			{MaxTokens: 1024 * 40, Multiplier: 1},
			{MaxTokens: 1024 * 50, Multiplier: 1},
			{MaxTokens: 1024 * 60, Multiplier: 1},
			{MaxTokens: 1024 * 70, Multiplier: 1},
			{MaxTokens: 1024 * 80, Multiplier: 1},
			{MaxTokens: 1024 * 90, Multiplier: 1},
			{MaxTokens: 1024 * 100, Multiplier: 1},
			{MaxTokens: 0, Multiplier: 1},
		},
		TextMultiplierBandsByCounter: map[string][]TextMultiplierBand{
			textCounterTiktoken: []TextMultiplierBand{
				{MaxTokens: 1024 * 10, Multiplier: 1},
				{MaxTokens: 1024 * 20, Multiplier: 1},
				{MaxTokens: 1024 * 30, Multiplier: 1},
				{MaxTokens: 1024 * 40, Multiplier: 1},
				{MaxTokens: 1024 * 50, Multiplier: 1},
				{MaxTokens: 1024 * 60, Multiplier: 1},
				{MaxTokens: 1024 * 70, Multiplier: 1},
				{MaxTokens: 1024 * 80, Multiplier: 1},
				{MaxTokens: 1024 * 90, Multiplier: 1},
				{MaxTokens: 1024 * 100, Multiplier: 1},
				{MaxTokens: 0, Multiplier: 1},
			},
			textCounterTextLen: []TextMultiplierBand{
				{MaxTokens: 1024 * 10, Multiplier: 1},
				{MaxTokens: 1024 * 20, Multiplier: 1},
				{MaxTokens: 1024 * 30, Multiplier: 1},
				{MaxTokens: 1024 * 40, Multiplier: 1},
				{MaxTokens: 1024 * 50, Multiplier: 1},
				{MaxTokens: 1024 * 60, Multiplier: 1},
				{MaxTokens: 1024 * 70, Multiplier: 1},
				{MaxTokens: 1024 * 80, Multiplier: 1},
				{MaxTokens: 1024 * 90, Multiplier: 1},
				{MaxTokens: 1024 * 100, Multiplier: 1},
				{MaxTokens: 0, Multiplier: 1},
			},
		},
		OverheadWeights: map[OverHeadItemKind]int{
			ItemPromptBase:         0,
			ItemRoleUser:           0,
			ItemRoleAssistant:      0,
			ItemRoleSystem:         0,
			ItemRoleTool:           0,
			ItemToolSection:        0,
			ItemToolDefinition:     4,
			ItemToolDescription:    1,
			ItemFunctionCall:       4,
			ItemFunctionCallResult: 4,

			ItemResponseFormatJsonSchema:                       14,
			ItemResponseFormatJsonSchemaStringPropertyRequired: 22,
			ItemThinkingBlock:                                  0,
			ItemHiddenReasoningBlock:                           0,
			ItemImageBlock:                                     0,
			ItemDocumentBlock:                                  0,
			ItemUnknownBlcok:                                   0,

			ItemToolPropertiesTypeString:        4,
			ItemToolPropertiesTypeNumber:        4,
			ItemToolPropertiesTypeInt:           4,
			ItemToolPropertiesTypeBool:          4,
			ItemToolPropertiesTypeObject:        3,
			ItemToolPropertiesTypeArray:         0,
			ItemToolPropertiesTypeArrayOfString: 4,

			ItemToolPropertiesDescription:           7,
			ItemToolRequired:                        0,
			ItemToolRequiredItem:                    0,
			ItemToolEnum:                            0,
			ItemToolEnumItem:                        1,
			ItemTooladditionalPropertiesBool:        0,
			ItemTooladditionalPropertiesTypeString:  0,
			ItemTooladditionalPropertiesTypeUnknown: 0,
		},
	},
	{
		API:                             apiGeminiStreamGenerateContent,
		ModelContains:                   []string{"gemini-2.5-pro"},
		TextCounter:                     "tiktoken",
		TextMultiplier:                  1,
		HiddenReasoningOutputMultiplier: 1.3,
		TextMultiplierBands: []TextMultiplierBand{
			{MaxTokens: 1024 * 10, Multiplier: 1},
			{MaxTokens: 1024 * 20, Multiplier: 1},
			{MaxTokens: 1024 * 30, Multiplier: 1},
			{MaxTokens: 1024 * 40, Multiplier: 1},
			{MaxTokens: 1024 * 50, Multiplier: 1},
			{MaxTokens: 1024 * 60, Multiplier: 1},
			{MaxTokens: 1024 * 70, Multiplier: 1},
			{MaxTokens: 1024 * 80, Multiplier: 1},
			{MaxTokens: 1024 * 90, Multiplier: 1},
			{MaxTokens: 1024 * 100, Multiplier: 1},
			{MaxTokens: 0, Multiplier: 1},
		},
		TextMultiplierBandsByCounter: map[string][]TextMultiplierBand{
			textCounterTiktoken: []TextMultiplierBand{
				{MaxTokens: 1024 * 10, Multiplier: 1},
				{MaxTokens: 1024 * 20, Multiplier: 1},
				{MaxTokens: 1024 * 30, Multiplier: 1},
				{MaxTokens: 1024 * 40, Multiplier: 1},
				{MaxTokens: 1024 * 50, Multiplier: 1},
				{MaxTokens: 1024 * 60, Multiplier: 1},
				{MaxTokens: 1024 * 70, Multiplier: 1},
				{MaxTokens: 1024 * 80, Multiplier: 1},
				{MaxTokens: 1024 * 90, Multiplier: 1},
				{MaxTokens: 1024 * 100, Multiplier: 1},
				{MaxTokens: 0, Multiplier: 1},
			},
			textCounterTextLen: []TextMultiplierBand{
				{MaxTokens: 1024 * 10, Multiplier: 1},
				{MaxTokens: 1024 * 20, Multiplier: 1},
				{MaxTokens: 1024 * 30, Multiplier: 1},
				{MaxTokens: 1024 * 40, Multiplier: 1},
				{MaxTokens: 1024 * 50, Multiplier: 1},
				{MaxTokens: 1024 * 60, Multiplier: 1},
				{MaxTokens: 1024 * 70, Multiplier: 1},
				{MaxTokens: 1024 * 80, Multiplier: 1},
				{MaxTokens: 1024 * 90, Multiplier: 1},
				{MaxTokens: 1024 * 100, Multiplier: 1},
				{MaxTokens: 0, Multiplier: 1},
			},
		},
		OverheadWeights: map[OverHeadItemKind]int{
			ItemPromptBase:         0,
			ItemRoleUser:           0,
			ItemRoleAssistant:      0,
			ItemRoleSystem:         0,
			ItemRoleTool:           0,
			ItemToolSection:        0,
			ItemToolDefinition:     0,
			ItemToolDescription:    0,
			ItemFunctionCall:       0,
			ItemFunctionCallResult: 0,

			ItemResponseFormatJsonSchema: 0,
			ItemThinkingBlock:            0,
			ItemHiddenReasoningBlock:     0,
			ItemImageBlock:               0,
			ItemDocumentBlock:            0,
			ItemUnknownBlcok:             0,

			ItemToolPropertiesTypeString:        1,
			ItemToolPropertiesTypeNumber:        1,
			ItemToolPropertiesTypeInt:           1,
			ItemToolPropertiesTypeBool:          1,
			ItemToolPropertiesTypeObject:        1,
			ItemToolPropertiesTypeArray:         0,
			ItemToolPropertiesTypeArrayOfString: 1,

			ItemToolPropertiesDescription:           0,
			ItemToolRequired:                        0,
			ItemToolRequiredItem:                    0,
			ItemToolEnum:                            0,
			ItemToolEnumItem:                        1,
			ItemTooladditionalPropertiesBool:        0,
			ItemTooladditionalPropertiesTypeString:  0,
			ItemTooladditionalPropertiesTypeUnknown: 0,
		},
	},
	{
		API:                             apiGeminiStreamGenerateContent,
		ModelContains:                   []string{"gemini-3-flash-preview"},
		TextCounter:                     "tiktoken",
		TextMultiplier:                  1,
		HiddenReasoningOutputMultiplier: 1.3,
		TextMultiplierBands: []TextMultiplierBand{
			{MaxTokens: 1024 * 10, Multiplier: 1},
			{MaxTokens: 1024 * 20, Multiplier: 1},
			{MaxTokens: 1024 * 30, Multiplier: 1},
			{MaxTokens: 1024 * 40, Multiplier: 1},
			{MaxTokens: 1024 * 50, Multiplier: 1},
			{MaxTokens: 1024 * 60, Multiplier: 1},
			{MaxTokens: 1024 * 70, Multiplier: 1},
			{MaxTokens: 1024 * 80, Multiplier: 1},
			{MaxTokens: 1024 * 90, Multiplier: 1},
			{MaxTokens: 1024 * 100, Multiplier: 1},
			{MaxTokens: 0, Multiplier: 1},
		},
		TextMultiplierBandsByCounter: map[string][]TextMultiplierBand{
			textCounterTiktoken: []TextMultiplierBand{
				{MaxTokens: 1024 * 10, Multiplier: 1},
				{MaxTokens: 1024 * 20, Multiplier: 1},
				{MaxTokens: 1024 * 30, Multiplier: 1},
				{MaxTokens: 1024 * 40, Multiplier: 1},
				{MaxTokens: 1024 * 50, Multiplier: 1},
				{MaxTokens: 1024 * 60, Multiplier: 1},
				{MaxTokens: 1024 * 70, Multiplier: 1},
				{MaxTokens: 1024 * 80, Multiplier: 1},
				{MaxTokens: 1024 * 90, Multiplier: 1},
				{MaxTokens: 1024 * 100, Multiplier: 1},
				{MaxTokens: 0, Multiplier: 1},
			},
			textCounterTextLen: []TextMultiplierBand{
				{MaxTokens: 1024 * 10, Multiplier: 1},
				{MaxTokens: 1024 * 20, Multiplier: 1},
				{MaxTokens: 1024 * 30, Multiplier: 1},
				{MaxTokens: 1024 * 40, Multiplier: 1},
				{MaxTokens: 1024 * 50, Multiplier: 1},
				{MaxTokens: 1024 * 60, Multiplier: 1},
				{MaxTokens: 1024 * 70, Multiplier: 1},
				{MaxTokens: 1024 * 80, Multiplier: 1},
				{MaxTokens: 1024 * 90, Multiplier: 1},
				{MaxTokens: 1024 * 100, Multiplier: 1},
				{MaxTokens: 0, Multiplier: 1},
			},
		},
		OverheadWeights: map[OverHeadItemKind]int{
			ItemPromptBase:         0,
			ItemRoleUser:           0,
			ItemRoleAssistant:      0,
			ItemRoleSystem:         0,
			ItemRoleTool:           0,
			ItemToolSection:        0,
			ItemToolDefinition:     4,
			ItemToolDescription:    1,
			ItemFunctionCall:       4,
			ItemFunctionCallResult: 4,

			ItemResponseFormatJsonSchema:                       14,
			ItemResponseFormatJsonSchemaStringPropertyRequired: 22,
			ItemThinkingBlock:                                  0,
			ItemHiddenReasoningBlock:                           0,
			ItemImageBlock:                                     0,
			ItemDocumentBlock:                                  0,
			ItemUnknownBlcok:                                   0,

			ItemToolPropertiesTypeString:        4,
			ItemToolPropertiesTypeNumber:        4,
			ItemToolPropertiesTypeInt:           4,
			ItemToolPropertiesTypeBool:          4,
			ItemToolPropertiesTypeObject:        3,
			ItemToolPropertiesTypeArray:         0,
			ItemToolPropertiesTypeArrayOfString: 4,

			ItemToolPropertiesDescription:           7,
			ItemToolRequired:                        0,
			ItemToolRequiredItem:                    0,
			ItemToolEnum:                            0,
			ItemToolEnumItem:                        1,
			ItemTooladditionalPropertiesBool:        0,
			ItemTooladditionalPropertiesTypeString:  0,
			ItemTooladditionalPropertiesTypeUnknown: 0,
		},
	},
	{
		API:                             apiMessages,
		ModelContains:                   []string{"claude-opus-4-8"},
		TextCounter:                     "tiktoken",
		TextMultiplier:                  1,
		HiddenReasoningOutputMultiplier: 1.3,
		TextMultiplierBands: []TextMultiplierBand{
			{MaxTokens: 1024 * 10, Multiplier: 1.65},
			{MaxTokens: 1024 * 20, Multiplier: 1.76},
			{MaxTokens: 1024 * 30, Multiplier: 1.77},
			{MaxTokens: 1024 * 40, Multiplier: 1.88},
			{MaxTokens: 1024 * 50, Multiplier: 1.1},
			{MaxTokens: 1024 * 60, Multiplier: 2.3},
			{MaxTokens: 1024 * 70, Multiplier: 2.08},
			{MaxTokens: 1024 * 80, Multiplier: 2.16},
			{MaxTokens: 1024 * 90, Multiplier: 2.23},
			{MaxTokens: 1024 * 100, Multiplier: 1},
			{MaxTokens: 0, Multiplier: 1.65},
		},
		TextMultiplierBandsByCounter: map[string][]TextMultiplierBand{
			textCounterTiktoken: []TextMultiplierBand{
				{MaxTokens: 1024 * 10, Multiplier: 1.65},
				{MaxTokens: 1024 * 20, Multiplier: 1.76},
				{MaxTokens: 1024 * 30, Multiplier: 1.77},
				{MaxTokens: 1024 * 40, Multiplier: 1.88},
				{MaxTokens: 1024 * 50, Multiplier: 1.1},
				{MaxTokens: 1024 * 60, Multiplier: 2.3},
				{MaxTokens: 1024 * 70, Multiplier: 2.08},
				{MaxTokens: 1024 * 80, Multiplier: 2.16},
				{MaxTokens: 1024 * 90, Multiplier: 2.23},
				{MaxTokens: 1024 * 100, Multiplier: 1},
				{MaxTokens: 0, Multiplier: 1.65},
			},
			textCounterTextLen: []TextMultiplierBand{
				{MaxTokens: 1024 * 10, Multiplier: 1.15},
				{MaxTokens: 1024 * 20, Multiplier: 1.46},
				{MaxTokens: 1024 * 30, Multiplier: 1.47},
				{MaxTokens: 1024 * 40, Multiplier: 1.58},
				{MaxTokens: 1024 * 50, Multiplier: 1.7},
				{MaxTokens: 1024 * 60, Multiplier: 2.0},
				{MaxTokens: 1024 * 70, Multiplier: 2.1},
				{MaxTokens: 1024 * 80, Multiplier: 2.14},
				{MaxTokens: 1024 * 90, Multiplier: 2.18},
				{MaxTokens: 1024 * 100, Multiplier: 2.2},
				{MaxTokens: 0, Multiplier: 2.4},
			},
		},
		OverheadWeights: map[OverHeadItemKind]int{
			ItemPromptBase:    5,
			ItemMessage:       0,
			ItemSystemMessage: 1,
			ItemRoleUser:      1,
			ItemRoleAssistant: 3,
			ItemRoleTool:      0,

			ItemFunctionCall:       16,
			ItemFunctionCallResult: 16,

			ItemCustomToolCall:       0,
			ItemCustomToolCallOutput: 0,
			ItemHiddenReasoningBlock: 300,
			ItemThinkingBlock:        0,
			ItemThinkingSignature:    1,
			ItemImageBlock:           200,
			ItemDocumentBlock:        0,
			ItemWebSearch:            0,
			ItemUnknownBlcok:         0,

			ItemToolSection:        290,
			ItemToolDefinition:     32,
			ItemToolChoiceAny:      120,
			ItemToolChoiceNone:     0,
			ItemToolChoiceAuto:     0,
			ItemToolChoiceToolName: 122,

			ItemToolPropertiesTypeString:            15,
			ItemToolPropertiesTypeObject:            18,
			ItemToolPropertiesTypeNumber:            10,
			ItemToolPropertiesTypeInt:               11,
			ItemToolPropertiesTypeBool:              12,
			ItemToolPropertiesTypeArray:             5,
			ItemToolPropertiesTypeArrayOfString:     12,
			ItemTooladditionalPropertiesTypeString:  17,
			ItemTooladditionalPropertiesTypeUnknown: 24,

			ItemToolPropertiesDescription:    9,
			ItemToolProperties:               9,
			ItemToolRequired:                 5,
			ItemToolRequiredItem:             2,
			ItemToolEnum:                     6,
			ItemToolEnumItem:                 2,
			ItemToolPropertiesItem:           1,
			ItemTooladditionalProperties:     10, // not finish
			ItemTooladditionalPropertiesBool: 10,

			ItemToolUseBlockInput:       19,
			ItemToolUseBlockInputItem:   3,
			ItemToolUseBlockInputString: 3,
			ItemToolUseBlockInputInt:    3,
			ItemToolUseBlockInputBool:   3,
			ItemToolUseBlockInputList:   3,
		},
	},
	{
		API:                             apiMessages,
		ModelContains:                   []string{"claude-opus-4-7"},
		TextCounter:                     "tiktoken",
		TextMultiplier:                  1,
		HiddenReasoningOutputMultiplier: 1.3,
		TextMultiplierBands: []TextMultiplierBand{
			{MaxTokens: 1024 * 10, Multiplier: 1.65},
			{MaxTokens: 1024 * 20, Multiplier: 1.76},
			{MaxTokens: 1024 * 30, Multiplier: 1.77},
			{MaxTokens: 1024 * 40, Multiplier: 1.88},
			{MaxTokens: 1024 * 50, Multiplier: 1.1},
			{MaxTokens: 1024 * 60, Multiplier: 2.3},
			{MaxTokens: 1024 * 70, Multiplier: 2.08},
			{MaxTokens: 1024 * 80, Multiplier: 2.16},
			{MaxTokens: 1024 * 90, Multiplier: 2.23},
			{MaxTokens: 1024 * 100, Multiplier: 1},
			{MaxTokens: 0, Multiplier: 1.65},
		},
		TextMultiplierBandsByCounter: map[string][]TextMultiplierBand{
			textCounterTiktoken: []TextMultiplierBand{
				{MaxTokens: 1024 * 10, Multiplier: 1.65},
				{MaxTokens: 1024 * 20, Multiplier: 1.76},
				{MaxTokens: 1024 * 30, Multiplier: 1.77},
				{MaxTokens: 1024 * 40, Multiplier: 1.88},
				{MaxTokens: 1024 * 50, Multiplier: 1.1},
				{MaxTokens: 1024 * 60, Multiplier: 2.3},
				{MaxTokens: 1024 * 70, Multiplier: 2.08},
				{MaxTokens: 1024 * 80, Multiplier: 2.16},
				{MaxTokens: 1024 * 90, Multiplier: 2.23},
				{MaxTokens: 1024 * 100, Multiplier: 1},
				{MaxTokens: 0, Multiplier: 1.65},
			},
			textCounterTextLen: []TextMultiplierBand{
				{MaxTokens: 1024 * 10, Multiplier: 1.15},
				{MaxTokens: 1024 * 20, Multiplier: 1.46},
				{MaxTokens: 1024 * 30, Multiplier: 1.47},
				{MaxTokens: 1024 * 40, Multiplier: 1.58},
				{MaxTokens: 1024 * 50, Multiplier: 1.7},
				{MaxTokens: 1024 * 60, Multiplier: 2.0},
				{MaxTokens: 1024 * 70, Multiplier: 2.1},
				{MaxTokens: 1024 * 80, Multiplier: 2.14},
				{MaxTokens: 1024 * 90, Multiplier: 2.18},
				{MaxTokens: 1024 * 100, Multiplier: 2.2},
				{MaxTokens: 0, Multiplier: 2.4},
			},
		},
		OutputTextMultiplierBandsByCounter: map[string][]TextMultiplierBand{
			textCounterTextLen: []TextMultiplierBand{
				{MaxTokens: 128, Multiplier: 1.15},
				{MaxTokens: 0, Multiplier: 1.84},
			},
		},
		OverheadWeights: map[OverHeadItemKind]int{
			ItemPromptBase:    5,
			ItemMessage:       0,
			ItemSystemMessage: 1,
			ItemRoleUser:      1,
			ItemRoleAssistant: 3,
			ItemRoleTool:      0,

			ItemFunctionCall:       40,
			ItemFunctionCallResult: 8,

			ItemCustomToolCall:       0,
			ItemCustomToolCallOutput: 0,
			ItemHiddenReasoningBlock: 300,
			ItemThinkingBlock:        0,
			ItemThinkingSignature:    1,
			ItemImageBlock:           0,
			ItemDocumentBlock:        0,
			ItemWebSearch:            0,
			ItemUnknownBlcok:         0,

			ItemToolSection:        290,
			ItemToolDefinition:     32,
			ItemToolChoiceAny:      120,
			ItemToolChoiceNone:     0,
			ItemToolChoiceAuto:     0,
			ItemToolChoiceToolName: 122,

			ItemToolPropertiesTypeString:            10,
			ItemToolPropertiesTypeObject:            18,
			ItemToolPropertiesTypeNumber:            10,
			ItemToolPropertiesTypeInt:               11,
			ItemToolPropertiesTypeBool:              12,
			ItemToolPropertiesTypeArray:             0,
			ItemToolPropertiesTypeArrayOfString:     12,
			ItemTooladditionalPropertiesTypeString:  17,
			ItemTooladditionalPropertiesTypeUnknown: 24,

			ItemToolPropertiesDescription:    5,
			ItemToolProperties:               7,
			ItemToolRequired:                 5,
			ItemToolRequiredItem:             2,
			ItemToolEnum:                     6,
			ItemToolEnumItem:                 2,
			ItemToolPropertiesItem:           1,
			ItemTooladditionalProperties:     10, // not finish
			ItemTooladditionalPropertiesBool: 10,

			ItemToolUseBlockInput:       35,
			ItemToolUseBlockInputItem:   0,
			ItemToolUseBlockInputString: 0,
			ItemToolUseBlockInputInt:    0,
			ItemToolUseBlockInputBool:   0,
			ItemToolUseBlockInputList:   0,
		},
		OverheadWeightOverrides: map[OverHeadItemKind]float64{
			ItemThinkingSignature: 0.25,
		},
	},
	{
		API:                             apiMessages,
		ModelContains:                   []string{"claude-sonnet-5"},
		TextCounter:                     "tiktoken",
		TextMultiplier:                  1,
		HiddenReasoningOutputMultiplier: 1.3,
		TextMultiplierBands: []TextMultiplierBand{
			{MaxTokens: 1024 * 10, Multiplier: 1.65},
			{MaxTokens: 1024 * 20, Multiplier: 1.76},
			{MaxTokens: 1024 * 30, Multiplier: 1.77},
			{MaxTokens: 1024 * 40, Multiplier: 1.88},
			{MaxTokens: 1024 * 50, Multiplier: 1.1},
			{MaxTokens: 1024 * 60, Multiplier: 2.3},
			{MaxTokens: 1024 * 70, Multiplier: 2.08},
			{MaxTokens: 1024 * 80, Multiplier: 2.16},
			{MaxTokens: 1024 * 90, Multiplier: 2.23},
			{MaxTokens: 1024 * 100, Multiplier: 1},
			{MaxTokens: 0, Multiplier: 1.65},
		},
		TextMultiplierBandsByCounter: map[string][]TextMultiplierBand{
			textCounterTiktoken: []TextMultiplierBand{
				{MaxTokens: 1024 * 10, Multiplier: 1.65},
				{MaxTokens: 1024 * 20, Multiplier: 1.76},
				{MaxTokens: 1024 * 30, Multiplier: 1.77},
				{MaxTokens: 1024 * 40, Multiplier: 1.88},
				{MaxTokens: 1024 * 50, Multiplier: 1.1},
				{MaxTokens: 1024 * 60, Multiplier: 2.3},
				{MaxTokens: 1024 * 70, Multiplier: 2.08},
				{MaxTokens: 1024 * 80, Multiplier: 2.16},
				{MaxTokens: 1024 * 90, Multiplier: 2.23},
				{MaxTokens: 1024 * 100, Multiplier: 1},
				{MaxTokens: 0, Multiplier: 1.65},
			},
			textCounterTextLen: []TextMultiplierBand{
				{MaxTokens: 1024 * 10, Multiplier: 1.15},
				{MaxTokens: 1024 * 20, Multiplier: 1.46},
				{MaxTokens: 1024 * 30, Multiplier: 1.47},
				{MaxTokens: 1024 * 40, Multiplier: 1.58},
				{MaxTokens: 1024 * 50, Multiplier: 1.7},
				{MaxTokens: 1024 * 60, Multiplier: 2.0},
				{MaxTokens: 1024 * 70, Multiplier: 2.1},
				{MaxTokens: 1024 * 80, Multiplier: 2.14},
				{MaxTokens: 1024 * 90, Multiplier: 2.18},
				{MaxTokens: 1024 * 100, Multiplier: 2.2},
				{MaxTokens: 0, Multiplier: 2.4},
			},
		},
		OverheadWeights: map[OverHeadItemKind]int{
			ItemPromptBase:    5,
			ItemMessage:       0,
			ItemSystemMessage: 1,
			ItemRoleUser:      1,
			ItemRoleAssistant: 3,
			ItemRoleTool:      0,

			ItemFunctionCall:       16,
			ItemFunctionCallResult: 8,

			ItemCustomToolCall:       0,
			ItemCustomToolCallOutput: 0,
			ItemHiddenReasoningBlock: 300,
			ItemThinkingBlock:        0,
			ItemThinkingSignature:    1,
			ItemImageBlock:           0,
			ItemDocumentBlock:        0,
			ItemWebSearch:            0,
			ItemUnknownBlcok:         0,

			ItemToolSection:        290,
			ItemToolDefinition:     32,
			ItemToolChoiceAny:      120,
			ItemToolChoiceNone:     0,
			ItemToolChoiceAuto:     0,
			ItemToolChoiceToolName: 122,

			ItemToolPropertiesTypeString:            10,
			ItemToolPropertiesTypeObject:            18,
			ItemToolPropertiesTypeNumber:            10,
			ItemToolPropertiesTypeInt:               11,
			ItemToolPropertiesTypeBool:              12,
			ItemToolPropertiesTypeArray:             0,
			ItemToolPropertiesTypeArrayOfString:     12,
			ItemTooladditionalPropertiesTypeString:  17,
			ItemTooladditionalPropertiesTypeUnknown: 24,

			ItemToolPropertiesDescription:    5,
			ItemToolProperties:               7,
			ItemToolRequired:                 5,
			ItemToolRequiredItem:             2,
			ItemToolEnum:                     6,
			ItemToolEnumItem:                 2,
			ItemToolPropertiesItem:           1,
			ItemTooladditionalProperties:     10, // not finish
			ItemTooladditionalPropertiesBool: 10,

			ItemToolUseBlockInput:       19,
			ItemToolUseBlockInputItem:   0,
			ItemToolUseBlockInputString: 0,
			ItemToolUseBlockInputInt:    0,
			ItemToolUseBlockInputBool:   0,
			ItemToolUseBlockInputList:   0,
		},
	},
	{
		API:                             apiMessages,
		ModelContains:                   []string{"claude-sonnet-4-6"},
		TextCounter:                     "tiktoken",
		TextMultiplier:                  1,
		HiddenReasoningOutputMultiplier: 1.3,
		TextMultiplierBands: []TextMultiplierBand{
			{MaxTokens: 1024 * 10, Multiplier: 1.65},
			{MaxTokens: 1024 * 20, Multiplier: 1.76},
			{MaxTokens: 1024 * 30, Multiplier: 1.77},
			{MaxTokens: 1024 * 40, Multiplier: 1.88},
			{MaxTokens: 1024 * 50, Multiplier: 1.1},
			{MaxTokens: 1024 * 60, Multiplier: 2.3},
			{MaxTokens: 1024 * 70, Multiplier: 2.08},
			{MaxTokens: 1024 * 80, Multiplier: 2.16},
			{MaxTokens: 1024 * 90, Multiplier: 2.23},
			{MaxTokens: 1024 * 100, Multiplier: 1},
			{MaxTokens: 0, Multiplier: 1.65},
		},
		TextMultiplierBandsByCounter: map[string][]TextMultiplierBand{
			textCounterTiktoken: []TextMultiplierBand{
				{MaxTokens: 1024 * 10, Multiplier: 1.65},
				{MaxTokens: 1024 * 20, Multiplier: 1.76},
				{MaxTokens: 1024 * 30, Multiplier: 1.77},
				{MaxTokens: 1024 * 40, Multiplier: 1.88},
				{MaxTokens: 1024 * 50, Multiplier: 1.1},
				{MaxTokens: 1024 * 60, Multiplier: 2.3},
				{MaxTokens: 1024 * 70, Multiplier: 2.08},
				{MaxTokens: 1024 * 80, Multiplier: 2.16},
				{MaxTokens: 1024 * 90, Multiplier: 2.23},
				{MaxTokens: 1024 * 100, Multiplier: 1},
				{MaxTokens: 0, Multiplier: 1.65},
			},
			textCounterTextLen: []TextMultiplierBand{
				{MaxTokens: 1024 * 10, Multiplier: 1.15},
				{MaxTokens: 1024 * 20, Multiplier: 1.46},
				{MaxTokens: 1024 * 30, Multiplier: 1.47},
				{MaxTokens: 1024 * 40, Multiplier: 1.58},
				{MaxTokens: 1024 * 50, Multiplier: 1.7},
				{MaxTokens: 1024 * 60, Multiplier: 2.0},
				{MaxTokens: 1024 * 70, Multiplier: 2.1},
				{MaxTokens: 1024 * 80, Multiplier: 2.14},
				{MaxTokens: 1024 * 90, Multiplier: 2.18},
				{MaxTokens: 1024 * 100, Multiplier: 2.2},
				{MaxTokens: 0, Multiplier: 2.4},
			},
		},
		OverheadWeights: map[OverHeadItemKind]int{
			ItemPromptBase:    5,
			ItemMessage:       0,
			ItemSystemMessage: 1,
			ItemRoleUser:      1,
			ItemRoleAssistant: 3,
			ItemRoleTool:      0,

			ItemFunctionCall:       16,
			ItemFunctionCallResult: 8,

			ItemCustomToolCall:       0,
			ItemCustomToolCallOutput: 0,
			ItemHiddenReasoningBlock: 300,
			ItemThinkingBlock:        0,
			ItemThinkingSignature:    1,
			ItemImageBlock:           0,
			ItemDocumentBlock:        0,
			ItemWebSearch:            0,
			ItemUnknownBlcok:         0,

			ItemToolSection:        290,
			ItemToolDefinition:     32,
			ItemToolChoiceAny:      120,
			ItemToolChoiceNone:     0,
			ItemToolChoiceAuto:     0,
			ItemToolChoiceToolName: 122,

			ItemToolPropertiesTypeString:            10,
			ItemToolPropertiesTypeObject:            18,
			ItemToolPropertiesTypeNumber:            10,
			ItemToolPropertiesTypeInt:               11,
			ItemToolPropertiesTypeBool:              12,
			ItemToolPropertiesTypeArray:             0,
			ItemToolPropertiesTypeArrayOfString:     12,
			ItemTooladditionalPropertiesTypeString:  17,
			ItemTooladditionalPropertiesTypeUnknown: 24,

			ItemToolPropertiesDescription:    5,
			ItemToolProperties:               7,
			ItemToolRequired:                 5,
			ItemToolRequiredItem:             2,
			ItemToolEnum:                     6,
			ItemToolEnumItem:                 2,
			ItemToolPropertiesItem:           1,
			ItemTooladditionalProperties:     10, // not finish
			ItemTooladditionalPropertiesBool: 10,

			ItemToolUseBlockInput:       19,
			ItemToolUseBlockInputItem:   0,
			ItemToolUseBlockInputString: 0,
			ItemToolUseBlockInputInt:    0,
			ItemToolUseBlockInputBool:   0,
			ItemToolUseBlockInputList:   0,
		},
	},
}
