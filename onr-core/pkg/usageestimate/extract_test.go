package usageestimate

import (
	"reflect"
	"strings"
	"testing"
)

func TestExtractOpenAPISchema_DecodesNestedToolParameters(t *testing.T) {
	raw := map[string]any{
		"type":        "object",
		"description": "Search hotels by destination and filters.",
		"required":    []any{"destination", "dates"},
		"properties": map[string]any{
			"destination": map[string]any{
				"type":        "string",
				"description": "City or region to search in.",
			},
			"dates": map[string]any{
				"type":        "object",
				"description": "Check-in and check-out dates.",
				"required":    []any{"check_in", "check_out"},
				"properties": map[string]any{
					"check_in": map[string]any{
						"type":        "string",
						"format":      "date",
						"description": "Check-in date.",
					},
					"check_out": map[string]any{
						"type":        "string",
						"format":      "date",
						"description": "Check-out date.",
					},
				},
			},
			"amenities": map[string]any{
				"type":        "array",
				"description": "Preferred amenities.",
				"items": map[string]any{
					"type": "string",
					"enum": []any{"wifi", "parking", "pool"},
				},
			},
			"location": map[string]any{
				"anyOf": []any{
					map[string]any{"type": "string", "description": "City name."},
					map[string]any{
						"type": "object",
						"properties": map[string]any{
							"lat": map[string]any{"type": "number"},
							"lng": map[string]any{"type": "number"},
						},
					},
				},
			},
			"metadata": map[string]any{
				"type":                 "object",
				"additionalProperties": true,
				"pattern":              "^[a-z_]+$",
			},
		},
	}

	ctx := NewEstimateContext("test-model", apiMessages, EstimateInput)
	schema := extracOpenAPISchema(ctx, raw)
	if got, want := schema.Type, "object"; got != want {
		t.Fatalf("type=%#v want=%q", got, want)
	}
	if got, want := schema.Description, "Search hotels by destination and filters."; got != want {
		t.Fatalf("description=%q want=%q", got, want)
	}
	if got, want := len(schema.Required), 2; got != want {
		t.Fatalf("required len=%d want=%d", got, want)
	}
	if got, want := len(schema.Properties), 5; got != want {
		t.Fatalf("properties len=%d want=%d", got, want)
	}

	dates := schema.Properties["dates"]
	if dates == nil {
		t.Fatalf("dates property missing")
	}
	if got, want := dates.Type, "object"; got != want {
		t.Fatalf("dates.type=%#v want=%q", got, want)
	}
	if got, want := dates.Properties["check_in"].Format, "date"; got != want {
		t.Fatalf("check_in.format=%q want=%q", got, want)
	}

	amenities := schema.Properties["amenities"]
	if amenities == nil || amenities.Items == nil {
		t.Fatalf("amenities items missing")
	}
	if got, want := amenities.Items.Type, "string"; got != want {
		t.Fatalf("amenities.items.type=%#v want=%q", got, want)
	}
	if got, want := len(amenities.Items.Enum), 3; got != want {
		t.Fatalf("amenities.items.enum len=%d want=%d", got, want)
	}

	location := schema.Properties["location"]
	if location == nil {
		t.Fatalf("location property missing")
	}
	if got, want := len(location.AnyOf), 2; got != want {
		t.Fatalf("location.anyOf len=%d want=%d", got, want)
	}
	if got, want := location.AnyOf[1].Properties["lat"].Type, "number"; got != want {
		t.Fatalf("location.anyOf[1].lat.type=%#v want=%q", got, want)
	}

	metadata := schema.Properties["metadata"]
	if metadata == nil {
		t.Fatalf("metadata property missing")
	}
	if got, want := metadata.AdditionalProperties, true; got != want {
		t.Fatalf("metadata.additionalProperties=%#v want=%v", got, want)
	}
	if got, want := metadata.Pattern, "^[a-z_]+$"; got != want {
		t.Fatalf("metadata.pattern=%q want=%q", got, want)
	}

	assertOverHead(t, ctx, ItemToolPropertiesDescription, 7)
	assertOverHead(t, ctx, ItemToolPropertiesTypeObject, 4)
	assertOverHead(t, ctx, ItemToolPropertiesTypeString, 5)
	assertOverHead(t, ctx, ItemToolPropertiesTypeNumber, 2)
	assertOverHead(t, ctx, ItemToolPropertiesTypeArrayOfString, 1)
	assertOverHead(t, ctx, ItemToolRequired, 2)
	assertOverHead(t, ctx, ItemToolRequiredItem, 4)
	assertOverHead(t, ctx, ItemToolEnum, 1)
	assertOverHead(t, ctx, ItemToolEnumItem, 3)
	assertOverHead(t, ctx, ItemTooladditionalPropertiesBool, 1)
}

func TestExtractOpenAPISchema_RecordsAdditionalPropertiesSchemaOverheads(t *testing.T) {
	ctx := NewEstimateContext("test-model", apiMessages, EstimateInput)
	schema := extracOpenAPISchema(ctx, map[string]any{
		"type": "object",
		"additionalProperties": map[string]any{
			"type": "string",
		},
	})

	if got, want := schema.AdditionalProperties, map[string]any{"type": "string"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("additionalProperties=%#v want=%#v", got, want)
	}
	assertOverHead(t, ctx, ItemToolPropertiesTypeObject, 1)
	assertOverHead(t, ctx, ItemTooladditionalPropertiesTypeString, 1)
}

func TestExtractOpenAPISchema_RecordsTypeVariantOverheads(t *testing.T) {
	ctx := NewEstimateContext("test-model", apiMessages, EstimateInput)
	schema := extracOpenAPISchema(ctx, map[string]any{
		"type": "object",
		"properties": map[string]any{
			"nullable_name": map[string]any{
				"type": []any{"string", "null"},
			},
			"count": map[string]any{
				"type": "integer",
			},
			"enabled": map[string]any{
				"type": "boolean",
			},
			"values": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "number",
				},
			},
		},
	})

	if got, want := len(schema.Properties), 4; got != want {
		t.Fatalf("properties len=%d want=%d", got, want)
	}
	assertOverHead(t, ctx, ItemToolPropertiesTypeObject, 1)
	assertOverHead(t, ctx, ItemToolPropertiesTypeString, 1)
	assertOverHead(t, ctx, ItemToolPropertiesTypeInt, 1)
	assertOverHead(t, ctx, ItemToolPropertiesTypeBool, 1)
	assertOverHead(t, ctx, ItemToolPropertiesTypeArray, 1)
	assertOverHead(t, ctx, ItemToolPropertiesTypeNumber, 1)
}

func TestExtractOpenAPISchema_RecordsCombinatorOverheads(t *testing.T) {
	ctx := NewEstimateContext("test-model", apiMessages, EstimateInput)
	schema := extracOpenAPISchema(ctx, map[string]any{
		"oneOf": []any{
			map[string]any{"type": "string", "description": "Plain value."},
			map[string]any{"type": "integer", "description": "Numeric value."},
		},
		"allOf": []any{
			map[string]any{
				"type": "object",
				"properties": map[string]any{
					"enabled": map[string]any{"type": "boolean"},
				},
			},
		},
	})

	if got, want := len(schema.OneOf), 2; got != want {
		t.Fatalf("oneOf len=%d want=%d", got, want)
	}
	if got, want := len(schema.AllOf), 1; got != want {
		t.Fatalf("allOf len=%d want=%d", got, want)
	}
	assertOverHead(t, ctx, ItemToolPropertiesDescription, 2)
	assertOverHead(t, ctx, ItemToolPropertiesTypeString, 1)
	assertOverHead(t, ctx, ItemToolPropertiesTypeInt, 1)
	assertOverHead(t, ctx, ItemToolPropertiesTypeObject, 1)
	assertOverHead(t, ctx, ItemToolPropertiesTypeBool, 1)
}

func TestExtractOpenAPISchema_RecordsUnknownAdditionalPropertiesOverheads(t *testing.T) {
	tests := []struct {
		name string
		raw  map[string]any
	}{
		{
			name: "schema without type",
			raw: map[string]any{
				"type":                 "object",
				"additionalProperties": map[string]any{"description": "Freeform value."},
			},
		},
		{
			name: "non schema value",
			raw: map[string]any{
				"type":                 "object",
				"additionalProperties": []any{"unexpected"},
			},
		},
		{
			name: "unsupported schema type",
			raw: map[string]any{
				"type":                 "object",
				"additionalProperties": map[string]any{"type": "array"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewEstimateContext("test-model", apiMessages, EstimateInput)
			_ = extracOpenAPISchema(ctx, tt.raw)

			assertOverHead(t, ctx, ItemToolPropertiesTypeObject, 1)
			assertOverHead(t, ctx, ItemTooladditionalPropertiesTypeUnknown, 1)
		})
	}
}

func TestExtractOpenAPISchema_EmptyOrInvalidInputDoesNotRecordOverheads(t *testing.T) {
	tests := []struct {
		name string
		raw  map[string]any
	}{
		{name: "nil schema", raw: nil},
		{name: "marshal error", raw: map[string]any{"bad": func() {}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewEstimateContext("test-model", apiMessages, EstimateInput)
			schema := extracOpenAPISchema(ctx, tt.raw)

			if !reflect.DeepEqual(schema, ToolSchema{}) {
				t.Fatalf("schema=%#v want empty schema", schema)
			}
			if len(ctx.OverHeadItems) != 0 {
				t.Fatalf("overheads=%#v want empty", ctx.OverHeadItems)
			}
		})
	}
}

func TestAggregateToolSchemaTextDoesNotMutateOverheads(t *testing.T) {
	ctx := NewEstimateContext("claude-opus-4-8", apiMessages, EstimateInput)
	schema := extracOpenAPISchema(ctx, map[string]any{
		"type":        "object",
		"description": "Search hotels.",
		"required":    []any{"destination"},
		"properties": map[string]any{
			"destination": map[string]any{
				"type":        "string",
				"description": "City name.",
				"enum":        []any{"Shanghai", "Tokyo"},
			},
		},
	})
	ctx.Tools = append(ctx.Tools, EstimateTool{Name: "search_hotels", Parameters: schema})
	before := cloneOverheads(ctx.OverHeadItems)

	tokenizer := mustCloseSourceTokenizer(t, ctx)
	_ = tokenizer.ApplyChatTemplate()

	if !reflect.DeepEqual(ctx.OverHeadItems, before) {
		t.Fatalf("overheads changed after text aggregation: before=%#v after=%#v", before, ctx.OverHeadItems)
	}
}

func TestAggregateToolSchemaTextIncludesCombinatorText(t *testing.T) {
	ctx := NewEstimateContext("claude-opus-4-8", apiMessages, EstimateInput)
	schema := extracOpenAPISchema(ctx, map[string]any{
		"anyOf": []any{
			map[string]any{
				"type":        "string",
				"description": "Use a city name.",
				"enum":        []any{"Shanghai"},
			},
		},
		"oneOf": []any{
			map[string]any{
				"type":        "object",
				"description": "Use coordinates.",
				"required":    []any{"lat"},
				"properties": map[string]any{
					"lat": map[string]any{"type": "number", "description": "Latitude."},
				},
			},
		},
		"allOf": []any{
			map[string]any{
				"type":        "object",
				"description": "Shared constraints.",
				"properties": map[string]any{
					"unit": map[string]any{"type": "string", "enum": []any{"metric"}},
				},
			},
		},
	})

	tokenizer := mustCloseSourceTokenizer(t, ctx)
	tokenizer.textBuilder = &strings.Builder{}
	tokenizer.aggregateToolSchemaText(&schema)
	got := tokenizer.textBuilder.String()

	assertContainsAll(t, got, "Use a city name.", "Shanghai", "Use coordinates.", "lat", "Latitude.", "Shared constraints.", "unit", "metric")
}

func TestAggregateMessagesRequestTextIncludesMessagesAndTools(t *testing.T) {
	ctx := NewEstimateContext("claude-opus-4-8", apiMessages, EstimateInput)
	schema := extracOpenAPISchema(ctx, map[string]any{
		"type":        "object",
		"description": "Weather lookup input.",
		"required":    []any{"city", "unit"},
		"properties": map[string]any{
			"city": map[string]any{
				"type":        "string",
				"description": "City name.",
				"enum":        []any{"Shanghai"},
			},
			"unit": map[string]any{
				"type":        "string",
				"description": "Temperature unit.",
			},
		},
	})
	ctx.Messages = []EstimateMessage{
		{
			Role: "system",
			Content: []EstimateMessagesContent{
				{Type: "text", Text: "You are a weather assistant."},
			},
		},
		{
			Role: "user",
			Content: []EstimateMessagesContent{
				{Type: "text", Text: "Find the weather."},
			},
		},
		{
			Role: "assistant",
			Content: []EstimateMessagesContent{
				{Type: "thinking", Text: "Need weather lookup."},
				{
					Type:      "tool_use",
					ID:        "tool_use_id_should_not_be_aggregated",
					Name:      "get_weather",
					Arguments: map[string]any{"location": "Shanghai", "days": float64(2), "alerts": true, "tags": []any{"forecast"}},
				},
			},
		},
		{
			Role: "user",
			Content: []EstimateMessagesContent{
				{Type: "tool_result", ID: "tool_result_id_should_not_be_aggregated", Text: "Cloudy."},
			},
		},
	}
	ctx.Tools = []EstimateTool{
		{Name: "weather_lookup_tool", Description: "Get weather by city.", Parameters: schema},
	}
	before := cloneOverheads(ctx.OverHeadItems)

	tokenizer := mustCloseSourceTokenizer(t, ctx)
	tokenizer.textBuilder = &strings.Builder{}
	tokenizer.aggregateMessagesRequestText()
	got := tokenizer.textBuilder.String()

	assertContainsAll(t, got,
		"You are a weather assistant.",
		"Find the weather.",
		"Need weather lookup.",
		"get_weather",
		"weather_lookup_tool",
		"location",
		"Shanghai",
		"days",
		"2",
		"alerts",
		"true",
		"tags",
		"forecast",
		"Cloudy.",
		"Get weather by city.",
		"Weather lookup input.",
		"city",
		"unit",
		"City name.",
		"Temperature unit.",
	)
	assertNotContains(t, got, "tool_use_id_should_not_be_aggregated")
	assertNotContains(t, got, "tool_result_id_should_not_be_aggregated")
	if !reflect.DeepEqual(ctx.OverHeadItems, before) {
		t.Fatalf("overheads changed after message text aggregation: before=%#v after=%#v", before, ctx.OverHeadItems)
	}
}

func cloneOverheads(in map[OverHeadItemKind]int) map[OverHeadItemKind]int {
	out := make(map[OverHeadItemKind]int, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func assertContainsAll(t *testing.T, got string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(got, want) {
			t.Fatalf("text %q does not contain %q", got, want)
		}
	}
}

func assertNotContains(t *testing.T, got string, want string) {
	t.Helper()
	if strings.Contains(got, want) {
		t.Fatalf("text %q contains %q", got, want)
	}
}
