package apitransform

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/apitypes"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/jsonutil"
)

// Minimax image generation mapping (/v1/image_generation). These builtins
// replicate the relay Go adaptor (internal/channel/adaptor/minimax) so
// OpenAI-compatible image generation requests can be routed to Minimax through
// the DSL config-file pipeline instead of a provider-specific Go adaptor.
//
// Minimax speaks neither OpenAI's request shape (it takes aspect_ratio or
// width/height instead of size, and "base64" instead of "b64_json") nor its
// response shape (data is an object holding parallel image_urls/image_base64
// arrays, and failures arrive inside an HTTP 200 as base_resp.status_code).
//
// Generic bounds — prompt presence and length, n range, response_format
// membership — are deliberately left to the request validation directives
// (req_required/req_len/req_range/req_enum), which already express them and
// report a failing path. Only the Minimax-specific reshaping lives here.

// minimaxImageAspectRatioSizes maps each Minimax aspect ratio to the pixel
// dimensions Minimax documents for it, so an OpenAI size given in pixels can be
// recognized as that ratio.
var minimaxImageAspectRatioSizes = map[string][2]int{
	"1:1":  {1024, 1024},
	"16:9": {1280, 720},
	"4:3":  {1152, 864},
	"3:2":  {1248, 832},
	"2:3":  {832, 1248},
	"3:4":  {864, 1152},
	"9:16": {720, 1280},
	"21:9": {1344, 576},
}

// minimaxImagePixelAspectRatios is the reverse lookup of
// minimaxImageAspectRatioSizes, keyed by normalized "<width>x<height>".
var minimaxImagePixelAspectRatios = func() map[string]string {
	out := make(map[string]string, len(minimaxImageAspectRatioSizes))
	for ratio, wh := range minimaxImageAspectRatioSizes {
		out[fmt.Sprintf("%dx%d", wh[0], wh[1])] = ratio
	}
	return out
}()

// Minimax accepts free-form dimensions only within these bounds.
const (
	minimaxImageMinDimension  = 512
	minimaxImageMaxDimension  = 2048
	minimaxImageDimensionStep = 8
)

// minimaxImageNormalizeSize lowercases, trims and removes spaces from an OpenAI
// size value. The Go adaptor compares the raw string, so " 1024X1024 " fails
// there even though it names a supported resolution; normalizing first is a
// deliberate fix rather than a replication.
func minimaxImageNormalizeSize(size string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(size)), " ", "")
}

// minimaxImageSize resolves an OpenAI size into either a Minimax aspect_ratio or
// an explicit width/height pair. Exactly one of the two is returned: an empty
// aspect ratio means width/height carry the answer.
//
// Resolution order mirrors the Go adaptor: a documented pixel size maps to its
// aspect ratio, anything else is parsed as <width>x<height>. Bare aspect ratios
// ("16:9") are additionally accepted because aspect_ratio is Minimax's own
// parameter and the Go adaptor rejecting them is a gap, not a rule.
func minimaxImageSize(size string) (aspectRatio string, width, height int, err error) {
	normalized := minimaxImageNormalizeSize(size)
	if normalized == "" {
		return "1:1", 0, 0, nil
	}
	if _, ok := minimaxImageAspectRatioSizes[normalized]; ok {
		return normalized, 0, 0, nil
	}
	if ratio, ok := minimaxImagePixelAspectRatios[normalized]; ok {
		return ratio, 0, 0, nil
	}

	parts := strings.Split(normalized, "x")
	if len(parts) != 2 {
		return "", 0, 0, newRequestMappingError(CodeRequestSizeNotSupported, "size",
			"invalid size format: %s. Supported: 1:1, 16:9, 9:16, 4:3, 3:4, 3:2, 2:3, 21:9, their pixel equivalents, or <width>x<height>", size)
	}
	width, werr := strconv.Atoi(parts[0])
	if werr != nil {
		return "", 0, 0, newRequestMappingError(CodeRequestSizeNotSupported, "size",
			"invalid width: %s", parts[0])
	}
	height, herr := strconv.Atoi(parts[1])
	if herr != nil {
		return "", 0, 0, newRequestMappingError(CodeRequestSizeNotSupported, "size",
			"invalid height: %s", parts[1])
	}
	if !minimaxImageValidDimension(width) || !minimaxImageValidDimension(height) {
		return "", 0, 0, newRequestMappingError(CodeRequestSizeNotSupported, "size",
			"width/height must be in [%d,%d] and a multiple of %d, got %dx%d",
			minimaxImageMinDimension, minimaxImageMaxDimension, minimaxImageDimensionStep, width, height)
	}
	return "", width, height, nil
}

func minimaxImageValidDimension(v int) bool {
	return v >= minimaxImageMinDimension && v <= minimaxImageMaxDimension && v%minimaxImageDimensionStep == 0
}

// minimaxImageResponseFormat translates the OpenAI response_format spelling into
// Minimax's. Minimax names the base64 form "base64"; "b64_json" is OpenAI's
// name for the same thing. Comparison is case-insensitive: the Go adaptor
// compares verbatim, which lets "B64_JSON" reach Minimax unchanged and be
// rejected upstream.
func minimaxImageResponseFormat(responseFormat string) string {
	switch strings.ToLower(strings.TrimSpace(responseFormat)) {
	case "":
		return "url"
	case "b64_json", "base64":
		return "base64"
	default:
		return strings.ToLower(strings.TrimSpace(responseFormat))
	}
}

// MapOpenAIImagesToMinimaxImageRequest builds a Minimax /v1/image_generation
// request from an OpenAI images.generations request root, mirroring the relay Go
// adaptor's convertImageRequest: prompt and model pass through, a missing n
// defaults to 1, response_format defaults to url, size becomes aspect_ratio or
// width/height, and seed/watermark are forwarded when present.
func MapOpenAIImagesToMinimaxImageRequest(root apitypes.JSONObject) (apitypes.JSONObject, error) {
	aspectRatio, width, height, err := minimaxImageSize(strings.TrimSpace(jsonutil.CoerceString(root["size"])))
	if err != nil {
		return nil, err
	}

	out := apitypes.JSONObject{
		"model":           jsonutil.CoerceString(root["model"]),
		"prompt":          jsonutil.CoerceString(root["prompt"]),
		"response_format": minimaxImageResponseFormat(jsonutil.CoerceString(root["response_format"])),
	}

	// An omitted n means "one image". The Go adaptor defaults it the same way in
	// convertImageRequest, but its ValidateImageRequest rejects n < 1 first and
	// an absent n decodes to 0, so the default is unreachable there and every
	// request without an explicit n is rejected; that contradiction is what is
	// not replicated.
	//
	// An explicitly sent n=0 is a different case and never reaches here: the
	// req_range min=1 rule rejects it, matching both the Go adaptor and the
	// OpenAI images API, which require n >= 1. The `n <= 0` test below is only
	// about the absent-field decode, not about honouring a literal 0.
	n := jsonutil.CoerceInt(root["n"])
	if n <= 0 {
		n = 1
	}
	out["n"] = n

	if aspectRatio != "" {
		out["aspect_ratio"] = aspectRatio
	} else {
		out["width"] = width
		out["height"] = height
	}

	if seed, ok := jsonutil.CoerceIntOK(root["seed"]); ok {
		out["seed"] = seed
	}
	if watermark, ok := root["watermark"].(bool); ok {
		out["aigc_watermark"] = watermark
	}
	return out, nil
}

// MapMinimaxImageToOpenAIImagesResponseObject converts a Minimax
// /v1/image_generation response object into an OpenAI images response object.
// Minimax returns data as an object holding parallel image_base64/image_urls
// arrays; OpenAI expects an array of per-image objects.
//
// Business failures arrive inside an HTTP 200 as base_resp.status_code != 0 and
// are reported with Minimax's own status_msg, matching the Go adaptor's
// ParseResponse. The `error_when` directive expresses the same detection, but it
// cannot carry the upstream message, and onr has no runtime consumer for it —
// ErrorWhenRule.Matches is called only by relay — so relying on it would leave
// the check inert on the onr side.
//
// Unlike the Go adaptor's ConvertToOpenAIFormat, which returns an empty data
// array when neither array holds anything, a response carrying no image is an
// error here: a success-shaped body with no image gives the client nothing to
// detect while still being billed.
func MapMinimaxImageToOpenAIImagesResponseObject(root apitypes.JSONObject) (apitypes.JSONObject, error) {
	if err := minimaxImageBaseRespError(root); err != nil {
		return nil, err
	}

	payload, _ := root["data"].(map[string]any)
	data := make([]any, 0, 2)

	// Minimax populates image_base64 or image_urls depending on the requested
	// response_format; prefer base64 when both are present, matching the Go
	// adaptor.
	for _, raw := range minimaxImageStrings(payload, "image_base64") {
		data = append(data, apitypes.JSONObject{"b64_json": raw})
	}
	if len(data) == 0 {
		for _, raw := range minimaxImageStrings(payload, "image_urls") {
			data = append(data, apitypes.JSONObject{"url": raw})
		}
	}

	if len(data) == 0 {
		return nil, &UpstreamResponseError{
			StatusCode: http.StatusInternalServerError,
			Type:       "server_error",
			Code:       "upstream_no_image",
			Message:    "No image data found in response",
		}
	}

	return apitypes.JSONObject{
		"created": time.Now().Unix(),
		"data":    data,
	}, nil
}

// minimaxImageBaseRespError reports a Minimax business failure carried inside an
// HTTP 200. Only a present, non-zero status_code counts: a missing base_resp
// leaves the response to the normal image extraction below.
func minimaxImageBaseRespError(root apitypes.JSONObject) error {
	baseResp, _ := root["base_resp"].(map[string]any)
	if baseResp == nil {
		return nil
	}
	statusCode, ok := jsonutil.CoerceIntOK(baseResp["status_code"])
	if !ok || statusCode == 0 {
		return nil
	}
	message := strings.TrimSpace(jsonutil.CoerceString(baseResp["status_msg"]))
	if message == "" {
		message = fmt.Sprintf("minimax returned status_code %d", statusCode)
	}
	return &UpstreamResponseError{
		// The Go adaptor reports every base_resp failure as a 400; keeping that
		// so both routes classify Minimax business errors the same way.
		StatusCode: http.StatusBadRequest,
		Type:       "invalid_request_error",
		Code:       fmt.Sprintf("minimax_%d", statusCode),
		Message:    message,
	}
}

// minimaxImageStrings returns the non-empty strings of the named array field.
func minimaxImageStrings(payload map[string]any, field string) []string {
	raw, _ := payload[field].([]any)
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if value := strings.TrimSpace(jsonutil.CoerceString(item)); value != "" {
			out = append(out, value)
		}
	}
	return out
}
