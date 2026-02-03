// Package dslconfig provides a small nginx-like DSL for describing upstream routing,
// request/response transformations, error mapping and usage extraction.
//
// # Public API surface (intended for reuse)
//
// The following identifiers are considered part of the reusable API and are
// expected to remain stable (source-compatible) within the same major version:
//
//   - Registry / ProviderFile:
//
//   - NewRegistry, DefaultRegistry
//
//   - (*Registry).ReloadFromDir, (*Registry).ListProviderNames, (*Registry).GetProvider
//
//   - ValidateProviderFile, ValidateProvidersDir
//
//   - Routing:
//
//   - ProviderRouting, RoutingMatch
//
//   - (ProviderRouting).HasMatch, (ProviderRouting).Apply
//
//   - Request transform:
//
//   - ProviderRequestTransform, MatchRequestTransform, RequestTransform
//
//   - (ProviderRequestTransform).Select
//
//   - (RequestTransform).Apply
//
//   - JSONOp and related helpers in json_ops.go
//
//   - Response / error mapping:
//
//   - ProviderResponse, MatchResponse, ResponseDirective
//
//   - ProviderError, MatchError, ErrorDirective (aliases in error.go)
//
//   - (ProviderResponse).Select (and the same selection behavior for ProviderError)
//
//   - Usage extraction:
//
//   - ProviderUsage, MatchUsage, UsageExtractConfig
//
//   - ParseUsageExpr, UsageExpr
//
//   - ExtractUsage
//
// # Host integration
//
// This package only depends on Go stdlib and other open-next-router/pkg packages.
// It does not import any host application packages (e.g. next-router), so that it
// can be reused as a library. Host applications should:
//
//   - Build and pass a *dslmeta.Meta (see pkg/dslmeta) for each request.
//   - Use ProviderRouting/RequestTransform to mutate meta (base_url, path, model mapping).
//   - Apply JSONOps to the request/response bodies as needed.
//   - Apply ProviderResponse / ProviderError selection at the boundary where the host
//     writes the final response to the client.
package dslconfig
