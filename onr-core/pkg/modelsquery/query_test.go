package modelsquery

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/httpclient/httpclienttest"
)

func TestQuery_CustomModelsConfig(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("path=%q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("auth header=%q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-4o-mini"},{"id":"gpt-4.1"}]}`))
	}))
	defer srv.Close()

	pf := dslconfig.ProviderFile{
		Routing: dslconfig.ProviderRouting{
			BaseURLExpr: `"` + srv.URL + `"`,
		},
		Headers: dslconfig.ProviderHeaders{
			Defaults: dslconfig.PhaseHeaders{
				Auth: []dslconfig.HeaderOp{
					{
						Op:        "header_set",
						NameExpr:  `"Authorization"`,
						ValueExpr: `concat("Bearer ", $channel.key)`,
					},
				},
			},
		},
		Models: dslconfig.ProviderModels{
			Defaults: dslconfig.ModelsQueryConfig{
				Mode:    "custom",
				Method:  "GET",
				Path:    "/v1/models",
				IDPaths: []string{"$.data[*].id"},
			},
		},
	}

	result, err := Query(context.Background(), Params{
		Provider: "openai",
		File:     pf,
		Meta: &dslmeta.Meta{
			API: "chat.completions",
		},
		APIKey: "test-key",
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if result == nil {
		t.Fatalf("Query returned nil result")
	}
	if len(result.IDs) != 2 || result.IDs[0] != "gpt-4o-mini" || result.IDs[1] != "gpt-4.1" {
		t.Fatalf("ids=%v", result.IDs)
	}
}

func TestQuery_UsesInjectedHTTPClient(t *testing.T) {
	fakeClient := httpclienttest.NewFakeDoer(t,
		httpclienttest.NewStringResponse(http.StatusOK, `{"data":[{"id":"fake-model"}]}`),
	)

	pf := dslconfig.ProviderFile{
		Headers: dslconfig.ProviderHeaders{
			Defaults: dslconfig.PhaseHeaders{
				Auth: []dslconfig.HeaderOp{
					{
						Op:        "header_set",
						NameExpr:  `"Authorization"`,
						ValueExpr: `concat("Bearer ", $channel.key)`,
					},
				},
			},
		},
		Models: dslconfig.ProviderModels{
			Defaults: dslconfig.ModelsQueryConfig{
				Mode:    "custom",
				Method:  "GET",
				Path:    "/v1/models",
				IDPaths: []string{"$.data[*].id"},
			},
		},
	}

	result, err := Query(context.Background(), Params{
		Provider:   "openai",
		File:       pf,
		Meta:       &dslmeta.Meta{API: "chat.completions"},
		APIKey:     "sk-test",
		BaseURL:    "https://example.test",
		HTTPClient: fakeClient,
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if result == nil {
		t.Fatalf("Query returned nil result")
	}
	if len(result.IDs) != 1 || result.IDs[0] != "fake-model" {
		t.Fatalf("ids=%v", result.IDs)
	}
	reqs := fakeClient.Requests()
	if len(reqs) != 1 {
		t.Fatalf("requests=%d", len(reqs))
	}
	if reqs[0].URL.String() != "https://example.test/v1/models" {
		t.Fatalf("unexpected request url: %s", reqs[0].URL.String())
	}
	if reqs[0].Header.Get("Authorization") != "Bearer sk-test" {
		t.Fatalf("unexpected Authorization header: %s", reqs[0].Header.Get("Authorization"))
	}
}

func TestQuery_EvaluatesTemplateModelsPath(t *testing.T) {
	fakeClient := httpclienttest.NewFakeDoer(t,
		httpclienttest.NewStringResponse(http.StatusOK, `{"publisherModels":[{"name":"publishers/google/models/gemini-2.5-flash"}]}`),
	)

	pf := dslconfig.ProviderFile{
		Models: dslconfig.ProviderModels{
			Defaults: dslconfig.ModelsQueryConfig{
				Mode:    "custom",
				Method:  "GET",
				Path:    "/v1beta1/publishers/google/models",
				IDPaths: []string{"$.publisherModels[*].name"},
				IDRegex: `^publishers/google/models/(.+)$`,
				Headers: []dslconfig.HeaderOp{
					{
						Op:        "header_set",
						NameExpr:  `"x-goog-user-project"`,
						ValueExpr: `template("${credential.project_id}")`,
					},
				},
			},
		},
	}

	result, err := Query(context.Background(), Params{
		Provider: "vertex",
		File:     pf,
		Meta: &dslmeta.Meta{
			CredentialProjectID: "vertex-project",
			ChannelLocation:     "us-central1",
		},
		BaseURL:    "https://aiplatform.googleapis.com",
		HTTPClient: fakeClient,
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(result.IDs) != 1 || result.IDs[0] != "gemini-2.5-flash" {
		t.Fatalf("ids=%v", result.IDs)
	}
	reqs := fakeClient.Requests()
	if len(reqs) != 1 {
		t.Fatalf("requests=%d", len(reqs))
	}
	wantURL := "https://aiplatform.googleapis.com/v1beta1/publishers/google/models"
	if got := reqs[0].URL.String(); got != wantURL {
		t.Fatalf("request url=%q want=%q", got, wantURL)
	}
	if got := reqs[0].Header.Get("x-goog-user-project"); got != "vertex-project" {
		t.Fatalf("x-goog-user-project=%q", got)
	}
}

func TestQuery_BedrockFoundationModelsSignsRequest(t *testing.T) {
	fakeClient := httpclienttest.NewFakeDoer(t,
		httpclienttest.NewStringResponse(http.StatusOK, `{"modelSummaries":[{"modelId":"anthropic.claude-3-5-sonnet-20240620-v1:0"}]}`),
	)

	pf := dslconfig.ProviderFile{
		Routing: dslconfig.ProviderRouting{
			Transport: "aws_sdk",
		},
		Models: dslconfig.ProviderModels{
			Defaults: dslconfig.ModelsQueryConfig{
				Mode:    "custom",
				Method:  "GET",
				Path:    "/foundation-models",
				IDPaths: []string{"$.modelSummaries[*].modelId"},
			},
		},
	}

	result, err := Query(context.Background(), Params{
		Provider:           "aws-bedrock",
		File:               pf,
		Meta:               &dslmeta.Meta{API: "chat.completions"},
		AWSAccessKeyID:     "AKIDEXAMPLE",
		AWSSecretAccessKey: "SECRETEXAMPLE",
		AWSSessionToken:    "SESSIONEXAMPLE",
		AWSRegion:          "us-east-1",
		HTTPClient:         fakeClient,
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(result.IDs) != 1 || result.IDs[0] != "anthropic.claude-3-5-sonnet-20240620-v1:0" {
		t.Fatalf("ids=%v", result.IDs)
	}
	reqs := fakeClient.Requests()
	if len(reqs) != 1 {
		t.Fatalf("requests=%d", len(reqs))
	}
	req := reqs[0]
	if got, want := req.URL.String(), "https://bedrock.us-east-1.amazonaws.com/foundation-models"; got != want {
		t.Fatalf("request url=%q want=%q", got, want)
	}
	auth := req.Header.Get("Authorization")
	if !strings.Contains(auth, "Credential=AKIDEXAMPLE/") || !strings.Contains(auth, "/us-east-1/bedrock/aws4_request") {
		t.Fatalf("authorization=%q", auth)
	}
	if got := req.Header.Get("X-Amz-Security-Token"); got != "SESSIONEXAMPLE" {
		t.Fatalf("X-Amz-Security-Token=%q", got)
	}
}

func TestQuery_BedrockInferenceProfilesFiltersToAnthropicMessagesModels(t *testing.T) {
	fakeClient := httpclienttest.NewFakeDoer(t,
		httpclienttest.NewStringResponse(http.StatusOK, `{"inferenceProfileSummaries":[
			{"inferenceProfileId":"deepseek.v3.2"},
			{"inferenceProfileId":"amazon.nova-pro-v1:0"},
			{"inferenceProfileId":"anthropic.claude-haiku-4-5-20251001-v1:0"},
			{"inferenceProfileId":"global.anthropic.claude-haiku-4-5-20251001-v1:0"},
			{"inferenceProfileId":"us.anthropic.claude-sonnet-4-20250514-v1:0"}
		]}`),
	)

	pf := dslconfig.ProviderFile{
		Routing: dslconfig.ProviderRouting{
			Transport: "aws_sdk",
		},
		Models: dslconfig.ProviderModels{
			Defaults: dslconfig.ModelsQueryConfig{
				Mode:         "custom",
				Method:       "GET",
				Path:         "/inference-profiles?typeEquals=SYSTEM_DEFINED",
				IDPaths:      []string{"$.inferenceProfileSummaries[*].inferenceProfileId"},
				IDAllowRegex: `^(anthropic|global\.anthropic|us\.anthropic|eu\.anthropic|apac\.anthropic|sa\.anthropic|ca\.anthropic|au\.anthropic|jp\.anthropic)\.`,
			},
		},
	}

	result, err := Query(context.Background(), Params{
		Provider:           "aws-bedrock",
		File:               pf,
		Meta:               &dslmeta.Meta{API: "chat.completions"},
		AWSAccessKeyID:     "AKIDEXAMPLE",
		AWSSecretAccessKey: "SECRETEXAMPLE",
		AWSRegion:          "us-east-1",
		HTTPClient:         fakeClient,
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	want := []string{
		"anthropic.claude-haiku-4-5-20251001-v1:0",
		"global.anthropic.claude-haiku-4-5-20251001-v1:0",
		"us.anthropic.claude-sonnet-4-20250514-v1:0",
	}
	if !reflect.DeepEqual(result.IDs, want) {
		t.Fatalf("ids=%v want %v", result.IDs, want)
	}
	reqs := fakeClient.Requests()
	if len(reqs) != 1 {
		t.Fatalf("requests=%d", len(reqs))
	}
	if got, want := reqs[0].URL.String(), "https://bedrock.us-east-1.amazonaws.com/inference-profiles?typeEquals=SYSTEM_DEFINED"; got != want {
		t.Fatalf("request url=%q want=%q", got, want)
	}
}

func TestQuery_BedrockModelsConvertsRuntimeEndpointOverride(t *testing.T) {
	fakeClient := httpclienttest.NewFakeDoer(t,
		httpclienttest.NewStringResponse(http.StatusOK, `{"modelSummaries":[{"modelId":"amazon.titan-text-lite-v1"}]}`),
	)

	pf := dslconfig.ProviderFile{
		Routing: dslconfig.ProviderRouting{
			Transport: "aws_sdk",
		},
		Models: dslconfig.ProviderModels{
			Defaults: dslconfig.ModelsQueryConfig{
				Mode:    "custom",
				Method:  "GET",
				Path:    "/foundation-models",
				IDPaths: []string{"$.modelSummaries[*].modelId"},
			},
		},
	}

	_, err := Query(context.Background(), Params{
		Provider:           "aws-bedrock",
		File:               pf,
		Meta:               &dslmeta.Meta{API: "chat.completions"},
		BaseURL:            "https://bedrock-runtime.us-west-2.amazonaws.com",
		AWSAccessKeyID:     "AKIDEXAMPLE",
		AWSSecretAccessKey: "SECRETEXAMPLE",
		AWSRegion:          "us-west-2",
		HTTPClient:         fakeClient,
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	reqs := fakeClient.Requests()
	if len(reqs) != 1 {
		t.Fatalf("requests=%d", len(reqs))
	}
	if got, want := reqs[0].URL.String(), "https://bedrock.us-west-2.amazonaws.com/foundation-models"; got != want {
		t.Fatalf("request url=%q want=%q", got, want)
	}
}

func TestQuery_BedrockOpenAIModelsUsesMantleEndpoint(t *testing.T) {
	fakeClient := httpclienttest.NewFakeDoer(t,
		httpclienttest.NewStringResponse(http.StatusOK, `{"data":[{"id":"openai.gpt-oss-120b-1:0"}]}`),
	)

	pf := dslconfig.ProviderFile{
		Routing: dslconfig.ProviderRouting{
			Transport: "aws_sdk",
		},
		Models: dslconfig.ProviderModels{
			Defaults: dslconfig.ModelsQueryConfig{
				Mode:    "custom",
				Method:  "GET",
				Path:    "/v1/models",
				IDPaths: []string{"$.data[*].id"},
			},
		},
	}

	result, err := Query(context.Background(), Params{
		Provider:           "aws-bedrock-mantle",
		File:               pf,
		Meta:               &dslmeta.Meta{API: "chat.completions"},
		AWSAccessKeyID:     "AKIDEXAMPLE",
		AWSSecretAccessKey: "SECRETEXAMPLE",
		AWSRegion:          "us-west-2",
		HTTPClient:         fakeClient,
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if !reflect.DeepEqual(result.IDs, []string{"openai.gpt-oss-120b-1:0"}) {
		t.Fatalf("ids=%v", result.IDs)
	}
	reqs := fakeClient.Requests()
	if len(reqs) != 1 {
		t.Fatalf("requests=%d", len(reqs))
	}
	if got, want := reqs[0].URL.String(), "https://bedrock-mantle.us-west-2.api.aws/v1/models"; got != want {
		t.Fatalf("request url=%q want=%q", got, want)
	}
	if auth := reqs[0].Header.Get("Authorization"); !strings.Contains(auth, "/us-west-2/bedrock/aws4_request") {
		t.Fatalf("authorization=%q", auth)
	}
}

func TestQuery_BedrockOpenAIModelsConvertsControlEndpointOverrideToMantle(t *testing.T) {
	fakeClient := httpclienttest.NewFakeDoer(t,
		httpclienttest.NewStringResponse(http.StatusOK, `{"data":[{"id":"openai.gpt-oss-120b-1:0"}]}`),
	)

	pf := dslconfig.ProviderFile{
		Routing: dslconfig.ProviderRouting{
			Transport: "aws_sdk",
		},
		Models: dslconfig.ProviderModels{
			Defaults: dslconfig.ModelsQueryConfig{
				Mode:    "custom",
				Method:  "GET",
				Path:    "/v1/models",
				IDPaths: []string{"$.data[*].id"},
			},
		},
	}

	_, err := Query(context.Background(), Params{
		Provider:           "aws-bedrock-mantle",
		File:               pf,
		Meta:               &dslmeta.Meta{API: "chat.completions"},
		BaseURL:            "https://bedrock.us-west-2.amazonaws.com",
		AWSAccessKeyID:     "AKIDEXAMPLE",
		AWSSecretAccessKey: "SECRETEXAMPLE",
		AWSRegion:          "us-west-2",
		HTTPClient:         fakeClient,
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	reqs := fakeClient.Requests()
	if len(reqs) != 1 {
		t.Fatalf("requests=%d", len(reqs))
	}
	if got, want := reqs[0].URL.String(), "https://bedrock-mantle.us-west-2.api.aws/v1/models"; got != want {
		t.Fatalf("request url=%q want=%q", got, want)
	}
}
