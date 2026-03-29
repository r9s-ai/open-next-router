package openai

import (
	"encoding/json"
	"strings"

	onraudio "github.com/r9s-ai/open-next-router/onr-core/pkg/providerusage/audio"
)

type imageResponse struct {
	Data []json.RawMessage `json:"data"`
}

type audioUsageResponse struct {
	Usage *struct {
		Seconds float64 `json:"seconds"`
	} `json:"usage,omitempty"`
}

type responsesOutputItem struct {
	Type   string `json:"type"`
	Status string `json:"status"`
}

type responsesBody struct {
	Output []responsesOutputItem `json:"output"`
}

type AudioTranslationQuantities struct {
	InputSeconds      float64
	InputMTokens      float64
	OutputKCharacters float64
	OutputMTokens     float64
}

type AudioSpeechQuantities struct {
	InputKCharacters float64
	InputMTokens     float64
	OutputSeconds    float64
	OutputMTokens    float64
}

// ImageCountFromResponseBody returns the number of images in a JSON response body.
func ImageCountFromResponseBody(body []byte) (float64, bool, error) {
	if len(body) == 0 {
		return 0, false, nil
	}
	var resp imageResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, false, err
	}
	if len(resp.Data) == 0 {
		return 0, false, nil
	}
	return float64(len(resp.Data)), true, nil
}

// AudioUsageSecondsFromResponseBody returns usage.seconds when the response body carries it.
func AudioUsageSecondsFromResponseBody(body []byte) (float64, bool, error) {
	if len(body) == 0 {
		return 0, false, nil
	}
	var resp audioUsageResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, false, err
	}
	if resp.Usage == nil || resp.Usage.Seconds <= 0 {
		return 0, false, nil
	}
	return resp.Usage.Seconds, true, nil
}

func CompletedWebSearchCallsFromResponseBody(body []byte) (float64, bool, error) {
	if len(body) == 0 {
		return 0, false, nil
	}
	var resp responsesBody
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, false, err
	}
	var total float64
	for _, item := range resp.Output {
		if strings.EqualFold(strings.TrimSpace(item.Type), "web_search_call") &&
			strings.EqualFold(strings.TrimSpace(item.Status), "completed") {
			total++
		}
	}
	if total <= 0 {
		return 0, false, nil
	}
	return total, true, nil
}

func AudioInputSeconds(usageType string, audioTokens int64, seconds int64) (float64, bool) {
	switch strings.ToLower(strings.TrimSpace(usageType)) {
	case "tokens":
		return float64(audioTokens) / (1250.0 / 60.0), true
	case "seconds", "duration":
		return float64(seconds), true
	default:
		return 0, false
	}
}

func AudioInputMTokens(usageType string, audioTokens int64, seconds int64) (float64, bool) {
	switch strings.ToLower(strings.TrimSpace(usageType)) {
	case "tokens":
		return float64(audioTokens) / 1000000.0, true
	case "seconds", "duration":
		return (1250.0 / 60.0) * float64(seconds) / 1000000.0, true
	default:
		return 0, false
	}
}

func AudioInputSecondsOrPayloads(payloads [][]byte, usageType string, audioTokens int64, seconds int64) float64 {
	if v, ok := AudioInputSeconds(usageType, audioTokens, seconds); ok {
		return v
	}
	return onraudio.SumDurationsFromPayloadsOrDefault(payloads, 1.0)
}

func AudioInputMTokensOrPayloads(payloads [][]byte, usageType string, audioTokens int64, seconds int64) float64 {
	if v, ok := AudioInputMTokens(usageType, audioTokens, seconds); ok {
		return v
	}
	return float64(onraudio.SumEstimatedTokensFromPayloadsOrDefault(payloads, 1.0)) / 1000000.0
}

func TextKCharacters(text string) float64 {
	var chars int64
	for range text {
		chars++
	}
	return float64(chars) / 1000.0
}

func TextMTokens(text string, estimate func(string) int) float64 {
	if estimate == nil {
		return 0
	}
	return float64(estimate(text)) / 1000000.0
}

func AudioTranslationDerivedQuantities(payloads [][]byte, usageType string, audioTokens int64, seconds int64, text string, estimateTextTokens func(string) int) AudioTranslationQuantities {
	return AudioTranslationQuantities{
		InputSeconds:      AudioInputSecondsOrPayloads(payloads, usageType, audioTokens, seconds),
		InputMTokens:      AudioInputMTokensOrPayloads(payloads, usageType, audioTokens, seconds),
		OutputKCharacters: TextKCharacters(text),
		OutputMTokens:     TextMTokens(text, estimateTextTokens),
	}
}

func AudioSpeechDerivedQuantities(inputText string, responseBody []byte, estimateInputTokens func(string) int) AudioSpeechQuantities {
	outputSeconds := onraudio.DurationFromBytesOrDefault(responseBody, 1.0)
	return AudioSpeechQuantities{
		InputKCharacters: TextKCharacters(inputText),
		InputMTokens:     TextMTokens(inputText, estimateInputTokens),
		OutputSeconds:    outputSeconds,
		OutputMTokens:    float64(onraudio.EstimatedTokensFromDuration(outputSeconds)) / 1000000.0,
	}
}
