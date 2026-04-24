package proxy

import (
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/ssemetrics"
)

// sseMetricsTap is a thin ONR-local alias over the shared onr-core SSE tap.
type sseMetricsTap = ssemetrics.Tap

// newSSEMetricsTap returns nil when agg is nil; otherwise it returns a non-nil tap.
func newSSEMetricsTap(agg *dslconfig.StreamMetricsAggregator) *sseMetricsTap {
	return ssemetrics.NewTap(agg)
}
