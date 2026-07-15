package ssecollect

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/jsonutil"
)

type Options struct{}

const maxChunkCollectIndex = 100

type Event struct {
	Event string
	ID    string
	Data  []byte
	Done  bool
}

func CollectByMode(ctx context.Context, mode string, reader io.Reader, opts Options) (map[string]any, error) {
	events, err := Parse(ctx, reader)
	if err != nil {
		return nil, err
	}
	switch NormalizeMode(mode) {
	case "openai_responses":
		return collectOpenAIResponses(events)
	case "anthropic_messages":
		return collectAnthropicMessages(events)
	case "gemini_generate_content":
		return collectGeminiGenerateContent(events)
	default:
		return nil, fmt.Errorf("unsupported sse_collect mode %q", mode)
	}
}

func NormalizeMode(mode string) string {
	return strings.ToLower(strings.TrimSpace(mode))
}

func SupportsMode(mode string) bool {
	switch NormalizeMode(mode) {
	case "openai_responses", "anthropic_messages", "gemini_generate_content":
		return true
	default:
		return false
	}
}

func Parse(ctx context.Context, r io.Reader) ([]Event, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	var out []Event
	var eventName, id string
	var dataLines [][]byte
	flush := func() {
		if len(dataLines) == 0 && eventName == "" && id == "" {
			return
		}
		data := bytes.Join(dataLines, []byte("\n"))
		ev := Event{Event: strings.TrimSpace(eventName), ID: strings.TrimSpace(id), Data: data}
		if strings.TrimSpace(string(data)) == "[DONE]" {
			ev.Done = true
		}
		if ev.Event == "" && len(data) > 0 && !ev.Done {
			var root map[string]any
			if json.Unmarshal(data, &root) == nil {
				ev.Event = jsonutil.CoerceScalarString(root["type"])
			}
		}
		out = append(out, ev)
		eventName, id = "", ""
		dataLines = nil
	}
	for sc.Scan() {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		line := sc.Bytes()
		line = bytes.TrimSuffix(line, []byte{'\r'})
		if len(line) == 0 {
			flush()
			continue
		}
		if line[0] == ':' {
			continue
		}
		field, value, ok := bytes.Cut(line, []byte{':'})
		if !ok {
			field = line
			value = nil
		} else if len(value) > 0 && value[0] == ' ' {
			value = value[1:]
		}
		switch string(field) {
		case "event":
			eventName = string(value)
		case "id":
			id = string(value)
		case "data":
			dataLines = append(dataLines, append([]byte(nil), value...))
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	flush()
	return out, nil
}

func eventJSON(ev Event) (map[string]any, error) {
	if ev.Done || len(bytes.TrimSpace(ev.Data)) == 0 {
		return nil, nil
	}
	var root map[string]any
	if err := json.Unmarshal(ev.Data, &root); err != nil {
		return nil, err
	}
	return root, nil
}

func eventType(ev Event, root map[string]any) string {
	if s := strings.TrimSpace(ev.Event); s != "" {
		return s
	}
	return strings.TrimSpace(jsonutil.CoerceScalarString(root["type"]))
}

func collectOpenAIResponses(events []Event) (map[string]any, error) {
	var response map[string]any
	itemsByIndex := map[int]map[string]any{}
	for _, ev := range events {
		root, err := eventJSON(ev)
		if err != nil {
			return nil, err
		}
		if root == nil {
			continue
		}
		typ := eventType(ev, root)
		switch typ {
		case "error", "response.failed":
			return nil, upstreamError(root)
		case "response.completed", "response.incomplete":
			if r, _ := root["response"].(map[string]any); r != nil {
				response = cloneMap(r)
			}
		case "response.output_item.done":
			item, _ := root["item"].(map[string]any)
			if item == nil {
				continue
			}
			idx, err := boundedChunkIndex(root["output_index"], len(itemsByIndex), "output_index")
			if err != nil {
				return nil, err
			}
			if v, ok := item["output_index"]; ok {
				idx, err = boundedChunkIndex(v, idx, "item.output_index")
				if err != nil {
					return nil, err
				}
			}
			itemsByIndex[idx] = cloneMap(item)
		}
	}
	if response == nil {
		return nil, errors.New("sse_collect openai_responses: final response event is missing")
	}
	if len(itemsByIndex) > 0 {
		indexes := make([]int, 0, len(itemsByIndex))
		for idx := range itemsByIndex {
			indexes = append(indexes, idx)
		}
		sort.Ints(indexes)
		output := make([]any, 0, len(indexes))
		for _, idx := range indexes {
			output = append(output, itemsByIndex[idx])
		}
		response["output"] = output
	}
	return response, nil
}

type anthropicBlock struct {
	block     map[string]any
	jsonParts strings.Builder
}

func collectAnthropicMessages(events []Event) (map[string]any, error) {
	var msg map[string]any
	blocks := map[int]*anthropicBlock{}
	seenStop := false
	for _, ev := range events {
		root, err := eventJSON(ev)
		if err != nil {
			return nil, err
		}
		if root == nil {
			continue
		}
		switch eventType(ev, root) {
		case "error":
			return nil, upstreamError(root)
		case "message_start":
			if m, _ := root["message"].(map[string]any); m != nil {
				msg = cloneMap(m)
				msg["content"] = []any{}
			}
		case "content_block_start":
			idx, err := boundedChunkIndex(root["index"], len(blocks), "index")
			if err != nil {
				return nil, err
			}
			block, _ := root["content_block"].(map[string]any)
			if block == nil {
				block = map[string]any{}
			}
			blocks[idx] = &anthropicBlock{block: cloneMap(block)}
		case "content_block_delta":
			idx, err := boundedChunkIndex(root["index"], 0, "index")
			if err != nil {
				return nil, err
			}
			b := blocks[idx]
			if b == nil {
				b = &anthropicBlock{block: map[string]any{}}
				blocks[idx] = b
			}
			delta, _ := root["delta"].(map[string]any)
			applyAnthropicDelta(b, delta)
		case "content_block_stop":
			idx, err := boundedChunkIndex(root["index"], 0, "index")
			if err != nil {
				return nil, err
			}
			if b := blocks[idx]; b != nil {
				finalizeAnthropicBlock(b)
			}
		case "message_delta":
			if msg == nil {
				msg = map[string]any{}
			}
			if delta, _ := root["delta"].(map[string]any); delta != nil {
				for _, k := range []string{"stop_reason", "stop_sequence"} {
					if v, ok := delta[k]; ok {
						msg[k] = v
					}
				}
			}
			if usage, _ := root["usage"].(map[string]any); usage != nil {
				msg["usage"] = mergeMaps(asMap(msg["usage"]), usage)
			}
		case "message_stop":
			seenStop = true
		}
	}
	if msg == nil {
		return nil, errors.New("sse_collect anthropic_messages: message_start event is missing")
	}
	indexes := make([]int, 0, len(blocks))
	for idx := range blocks {
		indexes = append(indexes, idx)
	}
	sort.Ints(indexes)
	content := make([]any, 0, len(indexes))
	for _, idx := range indexes {
		b := blocks[idx]
		finalizeAnthropicBlock(b)
		content = append(content, b.block)
	}
	msg["content"] = content
	if !seenStop {
		return nil, errors.New("sse_collect anthropic_messages: message_stop event is missing")
	}
	return msg, nil
}

func applyAnthropicDelta(b *anthropicBlock, delta map[string]any) {
	if delta == nil {
		return
	}
	switch jsonutil.CoerceScalarString(delta["type"]) {
	case "text_delta":
		b.block["text"] = jsonutil.CoerceScalarString(b.block["text"]) + jsonutil.CoerceScalarString(delta["text"])
	case "input_json_delta":
		b.jsonParts.WriteString(jsonutil.CoerceScalarString(delta["partial_json"]))
	case "thinking_delta":
		b.block["thinking"] = jsonutil.CoerceScalarString(b.block["thinking"]) + jsonutil.CoerceScalarString(delta["thinking"])
	case "signature_delta":
		b.block["signature"] = jsonutil.CoerceScalarString(delta["signature"])
	default:
		for k, v := range delta {
			if k != "type" {
				b.block[k] = v
			}
		}
	}
}

func finalizeAnthropicBlock(b *anthropicBlock) {
	if b == nil || b.jsonParts.Len() == 0 {
		return
	}
	raw := b.jsonParts.String()
	var obj any
	if json.Unmarshal([]byte(raw), &obj) == nil {
		b.block["input"] = obj
	} else {
		b.block["input"] = raw
	}
}

func collectGeminiGenerateContent(events []Event) (map[string]any, error) {
	out := map[string]any{}
	candidates := map[int]map[string]any{}
	for _, ev := range events {
		root, err := eventJSON(ev)
		if err != nil {
			return nil, err
		}
		if root == nil {
			continue
		}
		for _, k := range []string{"usageMetadata", "promptFeedback", "modelVersion", "responseId"} {
			if v, ok := root[k]; ok {
				out[k] = v
			}
		}
		arr, _ := root["candidates"].([]any)
		for pos, raw := range arr {
			in, _ := raw.(map[string]any)
			if in == nil {
				continue
			}
			idx, err := boundedChunkIndex(in["index"], pos, "index")
			if err != nil {
				return nil, err
			}
			dst := candidates[idx]
			if dst == nil {
				dst = map[string]any{"index": idx}
				candidates[idx] = dst
			}
			mergeGeminiCandidate(dst, in)
		}
	}
	if len(candidates) == 0 {
		return nil, errors.New("sse_collect gemini_generate_content: no candidates collected")
	}
	indexes := make([]int, 0, len(candidates))
	for idx := range candidates {
		indexes = append(indexes, idx)
	}
	sort.Ints(indexes)
	outCandidates := make([]any, 0, len(indexes))
	for _, idx := range indexes {
		outCandidates = append(outCandidates, candidates[idx])
	}
	out["candidates"] = outCandidates
	return out, nil
}

func mergeGeminiCandidate(dst, in map[string]any) {
	for _, k := range []string{"finishReason", "finishMessage", "safetyRatings", "citationMetadata", "groundingMetadata", "avgLogprobs", "logprobsResult"} {
		if v, ok := in[k]; ok {
			dst[k] = v
		}
	}
	inContent, _ := in["content"].(map[string]any)
	if inContent == nil {
		return
	}
	content := asMap(dst["content"])
	if content == nil {
		content = map[string]any{}
		dst["content"] = content
	}
	if role := strings.TrimSpace(jsonutil.CoerceScalarString(inContent["role"])); role != "" {
		content["role"] = role
	}
	parts, _ := content["parts"].([]any)
	inParts, _ := inContent["parts"].([]any)
	for _, rawPart := range inParts {
		part, _ := rawPart.(map[string]any)
		if part == nil {
			continue
		}
		if text := jsonutil.CoerceScalarString(part["text"]); text != "" {
			if len(parts) > 0 {
				last, _ := parts[len(parts)-1].(map[string]any)
				if last != nil {
					if prev := jsonutil.CoerceScalarString(last["text"]); prev != "" {
						last["text"] = prev + text
						continue
					}
				}
			}
		}
		parts = append(parts, cloneMap(part))
	}
	content["parts"] = parts
}

func upstreamError(root map[string]any) error {
	if errObj, _ := root["error"].(map[string]any); errObj != nil {
		if msg := strings.TrimSpace(jsonutil.CoerceScalarString(errObj["message"])); msg != "" {
			return fmt.Errorf("upstream SSE error: %s", msg)
		}
	}
	if msg := strings.TrimSpace(jsonutil.CoerceScalarString(root["message"])); msg != "" {
		return fmt.Errorf("upstream SSE error: %s", msg)
	}
	return fmt.Errorf("upstream SSE error: %v", root)
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func mergeMaps(base, overlay map[string]any) map[string]any {
	if base == nil {
		base = map[string]any{}
	}
	for k, v := range overlay {
		base[k] = v
	}
	return base
}

func coerceInt(v any, fallback int) int {
	if n, ok := jsonutil.CoerceIntOK(v); ok {
		return n
	}
	return fallback
}

func boundedChunkIndex(v any, fallback int, field string) (int, error) {
	idx := coerceInt(v, fallback)
	if idx < 0 || idx > maxChunkCollectIndex {
		return 0, fmt.Errorf("%w: %s %d outside allowed range 0..%d", ErrInvalidChunkPayload, field, idx, maxChunkCollectIndex)
	}
	return idx, nil
}
