package proxy

import (
	"net/http"
	"sync"
	"time"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/oauthclient"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/pricing"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/usageestimate"
	"github.com/r9s-ai/open-next-router/onr/internal/logx"
)

const (
	contentEncodingIdentity = "identity"
	contentEncodingGzip     = "gzip"
	contentTypeJSON         = "application/json"
)

type ProviderKey struct {
	Name               string
	Value              string
	BaseURLOverride    string
	CredentialFile     string
	Location           string
	AWSAccessKeyID     string
	AWSSecretAccessKey string
	AWSSessionToken    string
	AWSRegion          string
}

type Result struct {
	Provider       string
	ProviderKey    string
	ProviderSource string
	API            string
	Stream         bool
	Model          string
	Status         int
	LatencyMs      int64
	Usage          map[string]any
	UsageStage     string
	FinishReason   string
	Cost           map[string]any
	TTFTMs         int64
	TPS            float64
}

type Client struct {
	HTTP         *http.Client
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	Registry     *dslconfig.Registry
	UsageEst     *usageestimate.Config

	// ProxyByProvider maps provider name -> outbound HTTP proxy URL.
	// Example: {"openai": "http://127.0.0.1:7890"}.
	ProxyByProvider map[string]string

	OAuthTokenPersistEnabled bool
	OAuthTokenPersistDir     string

	mu          sync.Mutex
	httpByProxy map[string]*http.Client

	oauthMu     sync.Mutex
	oauthClient *oauthclient.Client

	pricingMu      sync.RWMutex
	pricing        *pricing.Resolver
	pricingEnabled bool

	SystemLogger *logx.SystemLogger
}
