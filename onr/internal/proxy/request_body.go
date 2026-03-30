package proxy

import (
	"github.com/r9s-ai/open-next-router/onr-core/pkg/requestcanon"
)

type RequestBodyInfo = requestcanon.Snapshot

func InspectRequestBody(bodyBytes []byte, contentType string, allowNonJSON bool) (RequestBodyInfo, error) {
	return requestcanon.Inspect(bodyBytes, contentType, requestcanon.InspectOptions{AllowNonJSON: allowNonJSON})
}
