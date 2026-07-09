package requesttransform

import (
	"errors"
	"net/http"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/requestvalidate"
)

func requiredBodyRule(path string, parts ...string) dslconfig.RequestValidationRule {
	return dslconfig.RequestValidationRule{
		Op:        dslconfig.ReqRuleRequired,
		Source:    dslconfig.ReqValidationSourceBody,
		Path:      path,
		PathParts: parts,
	}
}

func TestApply_ValidationRunsBeforeJSONOps(t *testing.T) {
	t.Parallel()

	body := []byte(`{"model":"gpt-4o"}`)
	value := map[string]any{"model": "gpt-4o"}

	// json_set would create $.messages; validation must fail first because it
	// runs against the client's original parameters.
	_, err := Apply(&dslmeta.Meta{}, "application/json", body, value, &dslconfig.RequestTransform{
		ValidationRules: []dslconfig.RequestValidationRule{requiredBodyRule("$.messages", "messages")},
		JSONOps: []dslconfig.JSONOp{
			{Op: "json_set", Path: "$.messages", ValueExpr: `"filled"`},
		},
	}, ApplyOptions{})
	var verr *requestvalidate.RequestValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected *RequestValidationError, got %T: %v", err, err)
	}
	if verr.PathOrName != "$.messages" {
		t.Fatalf("unexpected param: %#v", verr)
	}
	// The failed request must not leave transform side effects visible.
	if _, exists := value["messages"]; exists {
		t.Fatalf("json_set ran despite validation failure")
	}
}

func TestApply_ValidationRunsBeforeReqMap(t *testing.T) {
	t.Parallel()

	body := []byte(`{"model":"gpt-4o"}`)
	value := map[string]any{"model": "gpt-4o"}

	_, err := Apply(&dslmeta.Meta{}, "application/json", body, value, &dslconfig.RequestTransform{
		ValidationRules: []dslconfig.RequestValidationRule{requiredBodyRule("$.messages", "messages")},
		ReqMapMode:      "openai_chat_to_anthropic_messages",
	}, ApplyOptions{})
	var verr *requestvalidate.RequestValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected *RequestValidationError, got %T: %v", err, err)
	}
}

func TestApply_ValidationSeesMappedModel(t *testing.T) {
	t.Parallel()

	body := []byte(`{"model":"gpt-4o"}`)
	value := map[string]any{"model": "gpt-4o"}

	enum := dslconfig.RequestValidationRule{
		Op:            dslconfig.ReqRuleEnum,
		Source:        dslconfig.ReqValidationSourceBody,
		Path:          "$.model",
		PathParts:     []string{"model"},
		LiteralValues: []any{"gpt-4o-prod"},
	}
	result, err := Apply(&dslmeta.Meta{DSLModelMapped: "gpt-4o-prod"}, "application/json", body, value, &dslconfig.RequestTransform{
		ValidationRules: []dslconfig.RequestValidationRule{enum},
	}, ApplyOptions{})
	if err != nil {
		t.Fatalf("expected mapped model to pass enum, got %v", err)
	}
	if result.Value["model"] != "gpt-4o-prod" {
		t.Fatalf("unexpected model: %#v", result.Value["model"])
	}
}

func TestApply_HeaderQueryValidationWithNilRoot(t *testing.T) {
	t.Parallel()

	headers := http.Header{"X-Api-Flavor": []string{"beta"}}
	rules := []dslconfig.RequestValidationRule{
		{Op: dslconfig.ReqRuleRequired, Source: dslconfig.ReqValidationSourceHeader, Name: "x-api-flavor", CanonicalName: "X-Api-Flavor"},
		{Op: dslconfig.ReqRuleRequired, Source: dslconfig.ReqValidationSourceQuery, Name: "api-version"},
	}

	// Non-JSON body (nil root): header/query rules still run and pass.
	_, err := Apply(&dslmeta.Meta{}, "audio/mpeg", []byte("binary"), nil, &dslconfig.RequestTransform{
		ValidationRules: rules,
	}, ApplyOptions{RequestHeaders: headers, RawQuery: "api-version=1"})
	if err != nil {
		t.Fatalf("expected header/query rules to pass with nil root, got %v", err)
	}

	// Body rule with nil root fails with the json_body rule.
	_, err = Apply(&dslmeta.Meta{}, "audio/mpeg", []byte("binary"), nil, &dslconfig.RequestTransform{
		ValidationRules: []dslconfig.RequestValidationRule{requiredBodyRule("$.model", "model")},
	}, ApplyOptions{RequestHeaders: headers})
	var verr *requestvalidate.RequestValidationError
	if !errors.As(err, &verr) || verr.Rule != requestvalidate.RuleJSONBody {
		t.Fatalf("expected json_body failure, got %T: %v", err, err)
	}
}
