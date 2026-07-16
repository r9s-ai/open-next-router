# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Version management system with automatic release workflow
- GoReleaser configuration for multi-platform builds
- GitHub Actions workflows for automated releases
- Automatic changelog generation based on conventional commits
- `ClaudeUsageByModel.GetClaudeUsage()`: constructs a `*ClaudeUsage` from per-model token fields, excluding the `Iterations` slice. Enables billing callers to obtain a flat usage snapshot for each iteration without carrying the full iteration list.
- `OpenAIUsageByModel.GetUsage()`: constructs a `*OpenAIChatCompletionsUsage` from per-model fields, excluding the `Iterations` slice. Mirrors the Claude helper for OpenAI-format server-side fallback iteration billing.

---

<!-- Releases will be automatically added below by GoReleaser -->
