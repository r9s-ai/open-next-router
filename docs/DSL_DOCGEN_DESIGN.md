# DSL Doc Generation Design (i18n)

## Status

Proposed (MVP-first).

## Context

`DSL_SYNTAX.md` and `DSL_SYNTAX_CN.md` are currently maintained manually.
This is error-prone when directives evolve in `onr-core/pkg/dslconfig` and `validate.go`.

The repository already has a metadata-driven generator precedent:
- `onr-lsp/cmd/onr-tmgen` generates TextMate grammar from `dslconfig` metadata.

This document defines a practical path to generate DSL references (EN + ZH-CN) from one canonical structural source.

## Goals

- Generate directive reference sections for:
  - `DSL_SYNTAX.md`
  - `DSL_SYNTAX_CN.md`
- Keep runtime behavior explicit and DSL-driven (no implicit proxy rules).
- Make syntax changes auditable and CI-verifiable.
- Support localization without duplicating structural logic.

## Non-Goals

- Do not auto-generate all narrative chapters (conceptual semantics, migration guides, long examples).
- Do not replace parser/validator tests with docs tests.

## Single Source of Truth Model

Use a two-layer model:

1. Structural spec (canonical facts, language-agnostic)
2. Locale bundles (human-readable text per language)

The structural spec is the true source for syntax/constraints.
Locale bundles are presentation data only.

## Proposed Layout

```text
onr-core/pkg/dslspec/
  types.go                    # typed model for spec + locale bundle
  load.go                     # YAML loader + validation
  validate.go                 # cross-field checks
  schema.yaml                 # structural DSL facts (canonical)
  i18n/
    en.yaml                   # English text
    zh-CN.yaml                # Chinese text

cmd/onr-dsldocgen/
  main.go                     # render markdown + (optional) metadata adapter

DSL_SYNTAX.md
DSL_SYNTAX_CN.md
```

## Format Choice: YAML vs TOML vs XML

Recommended: `YAML`.

Reasons:
- Nested structures are first-class (blocks/directives/args/examples).
- Multi-line text for docs/i18n is ergonomic.
- Easy to diff/review for content teams.

Trade-offs and decision:
- XML: explicit but too verbose; high maintenance cost.
- TOML: clean for flat config; weaker ergonomics for deep nested docs + long text.
- YAML: best balance for this use case, but must enforce strict validation.

## Structural Spec (schema.yaml) - Suggested Fields

Top-level:
- `version`: spec version (e.g. `next-router/0.1`)
- `blocks`: block definitions and allowed placement
- `directives`: directive catalog

Per directive:
- `id`: stable key (e.g. `oauth_mode`)
- `name`: keyword shown in DSL
- `allowed_in`: block IDs (`top`, `auth`, `request`, ...)
- `kind`: `statement` | `block`
- `args`: ordered argument definitions
- `modes`: builtin mode values (if applicable)
- `repeatable`: bool
- `constraints`: machine tags (e.g. `requires:oauth_form_when_custom`)
- `defaults`: optional default values metadata (docs-facing)
- `since`: optional version marker

Per arg:
- `name`
- `type`: `expr|string|int|bool|enum|jsonpath|identifier`
- `required`: bool
- `enum`: optional values

## Locale Bundles

Locale files contain text by stable IDs, not by duplicated syntax.

Example keys:
- `directive.<id>.summary`
- `directive.<id>.details`
- `directive.<id>.example`
- `block.<id>.title`
- `block.<id>.notes`

This keeps translation changes independent from structural changes.

## Generation Scope (MVP)

Generate only marked regions in both markdown files:

```md
<!-- BEGIN GENERATED: directive-reference -->
... generated content ...
<!-- END GENERATED: directive-reference -->
```

Manual sections remain editable:
- conventions
- architecture/semantics narrative
- migration notes
- long-form provider examples

## Generator Behavior

`onr-dsldocgen` should:

1. Load structural spec + locale bundle.
2. Validate:
  - all `allowed_in` blocks exist
  - locale keys complete for required fields
  - enum/mode duplicates rejected
3. Render markdown for each locale.
4. Replace only generated markers in target files.
5. Exit non-zero if markers are missing.

Optional (phase-2):
- Render `dslconfig` metadata adapter output to reduce duplication with LSP hover metadata.

## CI/Developer Workflow

Add targets:

- `make dslspec-sync`
  - sync `schema.yaml` and i18n placeholders from runtime directive metadata
- `make dsl-docs-gen`
  - run generator for EN + ZH-CN
- `make dsl-docs-check`
  - run generator
  - fail if `git diff --exit-code` is non-empty

Suggested PR policy:
- Any DSL directive change must include regenerated docs diff.

## Migration Plan

1. Introduce `dslspec` model and loader with tests.
2. Port a small subset (e.g. `auth` block directives) to prove shape. (Completed in early MVP stage)
3. Implement marker-based generator.
4. Enable CI check.
5. Incrementally migrate remaining directives. (Completed by metadata-assisted sync)
6. (Optional) derive `dslconfig/metadata.go` from spec for full unification.

## Adding a New Directive (Operator Runbook)

Use `dslspec` as the entry point, but keep runtime semantics explicit in code.

1. Update structural spec:
  - add directive in `onr-core/pkg/dslspec/schema.yaml`
  - set `allowed_in`, args, modes/enums, repeatability, constraints tags
2. Update locale bundles:
  - add EN text in `onr-core/pkg/dslspec/i18n/en.yaml`
  - add ZH-CN text in `onr-core/pkg/dslspec/i18n/zh-CN.yaml`
3. Regenerate artifacts:
  - `metadata.go` (if phase-2 metadata generation is enabled)
  - `DSL_SYNTAX.md`
  - `DSL_SYNTAX_CN.md`
4. Implement runtime behavior explicitly:
  - parser support in `onr-core/pkg/dslconfig/parse_*.go`
  - validation rules in `onr-core/pkg/dslconfig/validate.go`
  - execution semantics in the corresponding runtime module
5. Add/adjust tests:
  - parse tests
  - validation tests
  - critical semantic/runtime tests

The generator should never auto-implement runtime behavior.

## CI Assertions for New Directives

Add hard checks to prevent spec/runtime drift:

- `dsl-docs-check`: generated docs are up to date (`git diff --exit-code` after generation).
- `spec-locale-check`: required locale keys exist for all directive IDs.
- `spec-impl-check`: each directive declared in spec is recognized by parser/validator allowlists.
- `mode-enum-check`: spec mode/enum values are consistent with runtime constants/allowlists.

These checks make `dslspec` the single entry for declaration, while parser/validator remain the runtime authority.

## Risk Controls

- Keep parser/validator as runtime authority.
- Add consistency tests:
  - spec directives vs parser-registered directives
  - spec mode values vs validation allowlists
- Start with partial generation to avoid large one-shot migration risk.

## Acceptance Criteria (MVP)

- Running one command updates both:
  - `DSL_SYNTAX.md`
  - `DSL_SYNTAX_CN.md`
- Generated sections are deterministic.
- CI fails when generated output is stale.
- New directive addition requires updating canonical spec + locale text, then regenerate.
