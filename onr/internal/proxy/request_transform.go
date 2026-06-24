package proxy

import (
	"fmt"
	"io"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/requestcanon"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/requesttransform"
)

func readRequestBody(gc *gin.Context, api string) (bodyBytes []byte, root map[string]any, model string, contentType string, err error) {
	if cachedBody, ok := gc.Get("onr.request_body"); ok {
		if bodyBytes, ok = cachedBody.([]byte); ok {
			if cachedRoot, ok := gc.Get("onr.request_root"); ok {
				root, _ = cachedRoot.(map[string]any)
			}
			if cachedModel, ok := gc.Get("onr.request_model"); ok {
				model = strings.TrimSpace(fmt.Sprintf("%v", cachedModel))
			}
			if cachedContentType, ok := gc.Get("onr.request_content_type"); ok {
				contentType = strings.TrimSpace(fmt.Sprintf("%v", cachedContentType))
			}
			return bodyBytes, root, model, contentType, nil
		}
	}

	bodyBytes, err = io.ReadAll(gc.Request.Body)
	if err != nil {
		return nil, nil, "", "", err
	}
	_ = gc.Request.Body.Close()
	contentType = gc.Request.Header.Get("Content-Type")

	info, err := requestcanon.Inspect(bodyBytes, contentType, requestcanon.InspectOptions{
		AllowNonJSON: requestcanon.AllowNonJSONRequestBodyAPI(api),
	})
	if err != nil {
		return bodyBytes, nil, "", contentType, err
	}
	root = info.Root
	model = strings.TrimSpace(info.Model)

	if root != nil {
		if strings.TrimSpace(model) == "" {
			if v, ok := root["model"].(string); ok {
				model = strings.TrimSpace(v)
			}
		}
	}
	if strings.TrimSpace(model) == "" {
		if m2, ok := parseGeminiModelFromPath(gc.Request.URL.Path); ok && strings.TrimSpace(m2) != "" {
			model = strings.TrimSpace(m2)
		}
	}
	return bodyBytes, root, model, contentType, nil
}

// selectRequestTransform requires a non-nil meta.
func selectRequestTransform(pf dslconfig.ProviderFile, meta *dslmeta.Meta) (*dslconfig.RequestTransform, bool) {
	t, ok := pf.Request.Select(meta)
	if !ok {
		return nil, false
	}

	t.Apply(meta)
	return t, true
}

// applyGeminiModelRewrite requires a non-nil meta.
func applyGeminiModelRewrite(api string, meta *dslmeta.Meta) {
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(api)), "gemini.") {
		return
	}
	if strings.TrimSpace(meta.DSLModelMapped) == "" {
		return
	}
	if newPath, ok := replaceGeminiModelInPath(meta.RequestURLPath, meta.DSLModelMapped); ok {
		meta.RequestURLPath = newPath
	}
}

// applyRequestTransform requires a non-nil meta, and t is non-nil when hasT is true.
func applyRequestTransform(meta *dslmeta.Meta, contentType, contentEncoding string, bodyBytes []byte, root map[string]any, t *dslconfig.RequestTransform, hasT bool) (requesttransform.Result, error) {
	if !hasT {
		return requesttransform.Result{
			Body:        bodyBytes,
			Value:       root,
			Root:        root,
			ContentType: strings.TrimSpace(contentType),
		}, nil
	}
	return requesttransform.Apply(meta, contentType, bodyBytes, root, t, requesttransform.ApplyOptions{
		ContentEncoding: contentEncoding,
		RequestHeaders:  meta.RequestHeaders,
	})
}
