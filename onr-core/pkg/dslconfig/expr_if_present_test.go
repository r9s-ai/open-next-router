package dslconfig

import (
	"bytes"
	"mime/multipart"
	"strings"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

func ifPresentMeta(t *testing.T, body string) *dslmeta.Meta {
	t.Helper()
	m := &dslmeta.Meta{
		OriginModelName:    "sora-2",
		DSLModelMapped:     "sora-2",
		RequestBody:        []byte(body),
		RequestContentType: "application/json",
	}
	return m
}

// The routing block cannot see the request body through any other variable, so
// this is the only way to pick an upstream path by request shape. PPIO splits
// video generation into -text2video and -img2video exactly this way.
func TestIfPresent_SelectsBranchByRequestBody(t *testing.T) {
	expr := `concat("/v3/async/", $request.model_mapped, "-", if_present("$.input_reference", "img2video", "text2video"))`

	cases := map[string]struct {
		body string
		want string
	}{
		"reference present": {`{"prompt":"a fox","input_reference":[{"b64":"AAA"}]}`, "/v3/async/sora-2-img2video"},
		"reference absent":  {`{"prompt":"a fox"}`, "/v3/async/sora-2-text2video"},
		// An explicitly empty array carries no value, so it must route the same
		// as an absent field: the caller sent no reference image either way.
		"reference empty": {`{"prompt":"a fox","input_reference":[]}`, "/v3/async/sora-2-text2video"},
		"reference null":  {`{"prompt":"a fox","input_reference":null}`, "/v3/async/sora-2-text2video"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := EvalStringExpr(expr, ifPresentMeta(t, tc.body))
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

// Branches are ordinary expressions, so they compose with the builtin variables
// rather than being restricted to literals.
func TestIfPresent_BranchesAreExpressions(t *testing.T) {
	expr := `if_present("$.input_reference", $request.model_mapped, "none")`
	if got := EvalStringExpr(expr, ifPresentMeta(t, `{"input_reference":[{"b64":"A"}]}`)); got != "sora-2" {
		t.Fatalf("got %q want the mapped model", got)
	}
	if got := EvalStringExpr(expr, ifPresentMeta(t, `{}`)); got != "none" {
		t.Fatalf("got %q want none", got)
	}
}

// A nil request body must not panic and must take the else branch: nothing was
// sent, so nothing is present.
func TestIfPresent_NoBodyTakesElseBranch(t *testing.T) {
	expr := `if_present("$.input_reference", "yes", "no")`
	if got := EvalStringExpr(expr, &dslmeta.Meta{}); got != "no" {
		t.Fatalf("got %q want no", got)
	}
}

// A malformed call must fail the provider file at load time. Left to runtime it
// would silently route every request to one branch, which looks like a working
// config until someone notices half the traffic hitting the wrong endpoint.
func TestIfPresent_MalformedCallsRejectedAtValidation(t *testing.T) {
	cases := map[string]struct{ expr, want string }{
		"too few args":    {`if_present("$.a", "x")`, "3 arguments"},
		"too many args":   {`if_present("$.a", "x", "y", "z")`, "3 arguments"},
		"unquoted path":   {`if_present($.a, "x", "y")`, "quoted json path"},
		"empty path":      {`if_present("", "x", "y")`, "must not be empty"},
		"path without $":  {`if_present("input_reference", "x", "y")`, "must start with $"},
		"bad branch expr": {`if_present("$.a", $nope, "y")`, "branch"},
		"bad else branch": {`if_present("$.a", "x", $nope)`, "branch"},
		"nested bad path": {`if_present("$.a", "x", if_present("", "y", "z"))`, "branch"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := ValidateStringExpr(tc.expr)
			if err == nil {
				t.Fatalf("expected %q to be rejected", tc.expr)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err=%q should mention %q", err, tc.want)
			}
		})
	}
}

// In set_path both branches end up in the URL, so each has to be path-shaped on
// its own — a branch that is a bare word would produce a relative URL.
func TestIfPresent_SetPathRequiresPathShapedBranches(t *testing.T) {
	ok := `if_present("$.input_reference", "/v3/img2video", "/v3/text2video")`
	if err := validateSetPathExpr(ok); err != nil {
		t.Fatalf("validateSetPathExpr(%s): %v", ok, err)
	}

	bad := `if_present("$.input_reference", "img2video", "/v3/text2video")`
	err := validateSetPathExpr(bad)
	if err == nil {
		t.Fatal("a non-path branch should be rejected in set_path")
	}
	if !strings.Contains(err.Error(), "branch") {
		t.Fatalf("err=%q should name the offending branch", err)
	}
}

// Emptiness, not key presence, is what decides the branch — an empty string or
// empty object means the caller sent nothing, same as omitting the field.
func TestIfPresent_EmptyValuesCountAsAbsent(t *testing.T) {
	expr := `if_present("$.ref", "yes", "no")`
	for name, body := range map[string]string{
		"empty string": `{"ref":""}`,
		"blank string": `{"ref":"   "}`,
		"empty object": `{"ref":{}}`,
		"empty array":  `{"ref":[]}`,
		"null":         `{"ref":null}`,
	} {
		t.Run(name, func(t *testing.T) {
			if got := EvalStringExpr(expr, ifPresentMeta(t, body)); got != "no" {
				t.Fatalf("got %q want no for %s", got, body)
			}
		})
	}
	for name, body := range map[string]string{
		"non-empty string": `{"ref":"x"}`,
		"number":           `{"ref":0}`,
		"false":            `{"ref":false}`,
		"non-empty array":  `{"ref":[1]}`,
	} {
		t.Run(name, func(t *testing.T) {
			if got := EvalStringExpr(expr, ifPresentMeta(t, body)); got != "yes" {
				t.Fatalf("got %q want yes for %s", got, body)
			}
		})
	}
}

// Routing runs before req_inline_file, so at set_path time an uploaded file is
// not in the request root at all — it was discarded during canonicalization.
// Without consulting the multipart file fields, an image-to-video request would
// be routed to the text-to-video endpoint.
func TestIfPresent_SeesMultipartUploadsBeforeInlining(t *testing.T) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if err := w.WriteField("prompt", "a fox"); err != nil {
		t.Fatalf("WriteField: %v", err)
	}
	fw, err := w.CreateFormFile("input_reference", "ref.png")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := fw.Write([]byte("PNGBYTES")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	m := &dslmeta.Meta{
		OriginModelName:    "sora-2",
		DSLModelMapped:     "sora-2",
		RequestBody:        buf.Bytes(),
		RequestContentType: w.FormDataContentType(),
	}
	// The upload is absent from the root, which is exactly why the file-field
	// lookup is needed.
	if _, exists := m.RequestRoot()["input_reference"]; exists {
		t.Fatal("canonicalization should not put uploads in the request root")
	}

	expr := `concat("/v3/async/", $request.model_mapped, "-", if_present("$.input_reference", "img2video", "text2video"))`
	if got := EvalStringExpr(expr, m); got != "/v3/async/sora-2-img2video" {
		t.Fatalf("got %q want the image-to-video path", got)
	}
}

// A multipart request without the upload still takes the else branch.
func TestIfPresent_MultipartWithoutUploadTakesElseBranch(t *testing.T) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if err := w.WriteField("prompt", "a fox"); err != nil {
		t.Fatalf("WriteField: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	m := &dslmeta.Meta{
		OriginModelName:    "sora-2",
		DSLModelMapped:     "sora-2",
		RequestBody:        buf.Bytes(),
		RequestContentType: w.FormDataContentType(),
	}
	expr := `if_present("$.input_reference", "img2video", "text2video")`
	if got := EvalStringExpr(expr, m); got != "text2video" {
		t.Fatalf("got %q want text2video", got)
	}
}
