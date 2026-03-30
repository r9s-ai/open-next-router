# onr-core `pkg/`

This directory contains the public, reusable building blocks of `onr-core`.
These packages are intended to be shared by ONR itself and by downstream integrations such as `relay`.

## Design intent

- Keep provider-agnostic runtime semantics in `pkg/`.
- Expose stable libraries for parsing, DSL evaluation, request transformation, pricing, and diagnostics.
- Avoid coupling these packages to Gin handlers, server wiring, or project-specific controller flows.

## Package index

| Package | Purpose |
| --- | --- |
| `apitransform` | Built-in request/response schema mappers between provider APIs, such as OpenAI, Anthropic, and Gemini. |
| `apitypes` | Shared typed request/response structures for supported upstream API families. |
| `appnameinfer` | Heuristics for inferring an app or product name from request context. |
| `balancequery` | Reusable balance querying logic driven by DSL configuration. |
| `dslconfig` | Core DSL parser, validator, selector, JSON ops, and runtime config execution logic. |
| `dslmeta` | Minimal metadata model consumed by the DSL engine and related helpers. |
| `dslspec` | DSL directive metadata used by docs, tooling, and editor integrations. |
| `httpclient` | Small shared HTTP client abstractions used by reusable runtime helpers. |
| `jsonutil` | Generic JSON value helpers used across DSL and runtime code. |
| `keystore` | Provider key storage and selection helpers. |
| `models` | Model routing file loading and in-memory model-to-provider router. |
| `modelsquery` | Provider-side model discovery logic based on DSL `models` configuration. |
| `oauthclient` | Shared OAuth token acquisition and refresh helpers for upstream access. |
| `pricing` | Pricing catalog loading, normalization, and runtime lookup helpers. |
| `providerusage` | Provider-specific usage extraction helpers that do not belong in server wiring. |
| `requestcanon` | Canonical request inspection for request body bytes, request root, model, stream, and content type. |
| `requestid` | Shared request ID utilities and header normalization helpers. |
| `requesttransform` | Canonical request-side transform pipeline for JSON ops, req_map, and body rebuilding. |
| `trafficdump` | Reusable request/response dump helpers for diagnostics and debugging. |
| `usageestimate` | Heuristics for request-side usage estimation when upstream usage is missing or delayed. |

## `providerusage` subpackages

| Package | Purpose |
| --- | --- |
| `providerusage/audio` | Audio-specific derived usage helpers, such as duration-based signals. |
| `providerusage/openai` | OpenAI-family usage extraction helpers shared across runtimes. |

## Rules of thumb

- If the logic is provider-agnostic and reusable by both ONR and `relay`, it likely belongs under `pkg/`.
- If the logic depends on server wiring, Gin context, controller orchestration, or project-specific policy, keep it outside `pkg/`.
- New packages should expose narrow, composable APIs rather than large framework-style entrypoints.
