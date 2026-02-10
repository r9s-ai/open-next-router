## Communication

- Keep change notes verifiable: prefer concrete file paths and entry points.

## Project Goal (Important)

open-next-router (ONR) is centered around a **nginx-like, atomic DSL configuration** model:

- Runtime behavior must be explicitly described by the DSL in `config/providers/*.conf`.
- The code layer (especially `internal/proxy`) should remain an execution engine:
  - **No implicit guessing / auto-compat** based on `path/api/model`.
  - Do not introduce hidden rules like “if path contains X then transform”.
- When adding capabilities, prefer adding **atomic DSL directives** (e.g. `req_map/resp_map/sse_parse`) and implement
  parsing/validation/semantics in `onr-core/pkg/dslconfig`, then execute strictly according to the selected directives.

## Directories & Boundaries

- Runtime CLI logic (entry glue, config test, signals): `onr/*`
- Runtime server: `onr/internal/onrserver/*`
- Proxy/execution engine: `onr/internal/proxy/*`
- Runtime-only shared utilities: `onr/internal/{auth,logx,requestid}/*`
- Version/build info (shared): `internal/version/*`
- Admin implementation (not for external import): `onr-admin/internal/{cli,store,tui}/*`
- Admin public facade (imported by `cmd/onr-admin`): `onr-admin/cli/*`
- Core reusable library: `onr-core/pkg/*`
- DSL (parsing/validation/semantic model/reusable utilities): `onr-core/pkg/dslconfig/*`
- Provider configs: `config/providers/*.conf`

Boundary rules:

- Put reusable, HTTP/Gin-free logic in `onr-core/pkg/*` when possible (e.g. JSON/SSE converters).
- `onr/internal/*` and `onr-admin/*` may depend on `onr-core/pkg/*`; the reverse is not allowed.
- Respect Go `internal/` import boundaries:
  - `onr/internal/*` may only be imported by code under `onr/`.
  - `onr-admin/internal/*` may only be imported by code under `onr-admin/`.

## DSL Change Checklist

When adding/modifying a DSL directive, you must also do:

1. **Parsing**: add syntax support in `onr-core/pkg/dslconfig/parse_*.go` (nginx-style; every statement ends with `;`).
2. **Validation**: add allowlist/constraints in `onr-core/pkg/dslconfig/validate.go` to keep runtime behavior controlled.
3. **Docs**: update `DSL_SYNTAX.md` with syntax, semantics, examples, and limitations.
4. **Tests**: add/update `onr-core/pkg/dslconfig/*_test.go` to cover parsing + critical semantics.

## Compatibility / Transformation Features

- Compatibility must be **opt-in and explicit** via DSL (e.g. `req_map ...;`, `resp_map ...;`, `sse_parse ...;`).
- Do not implement compatibility by adding implicit rules in `onr/internal/proxy`.
- SSE transformations must handle: `event:`/`data:` framing, blank-line flush, `data: [DONE]`,
  and tool-call event incremental/aggregation semantics.

## Language & Docs

- Source code (identifiers, comments), CLI help/output, and documentation must be written in English.
- Chinese documentation is allowed only as a separate file with the `_CN` suffix (e.g. `DSL_SYNTAX_CN.md`).

## Development & Verification

- Go code must pass `gofmt`.
- After changing provider configs (`config/providers/*.conf`), keep or add a test to validate the providers directory
  can be parsed and validated (e.g. `ValidateProvidersDir`).
