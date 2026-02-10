# Release Guide

This repository contains two release tracks:

1. Runtime/CLI release (`onr`, `onr-admin`, Docker image)
2. `onr-core` Go module release for external consumers

## 1) Runtime/CLI release

Use a root tag in the format `vX.Y.Z`:

```bash
git tag -a v1.2.3 -m "Release v1.2.3"
git push origin v1.2.3
```

This triggers `.github/workflows/release.yml`, which runs tests and GoReleaser.

Outputs:

- GitHub release assets for `onr` and `onr-admin`
- Config bundle: `open-next-router_config_vX.Y.Z.tar.gz` (includes `config/providers/*.conf` and `config/*.example.yaml`)
- GHCR image: `ghcr.io/r9s-ai/open-next-router:<version>` and `latest`

## 2) onr-core module release (stable external import)

`onr-core` is a separate Go module:

- Module path: `github.com/r9s-ai/open-next-router/onr-core`
- Module file: `onr-core/go.mod`

To publish a stable module version, use a submodule tag in the format `onr-core/vX.Y.Z`:

```bash
git tag -a onr-core/v1.2.3 -m "onr-core v1.2.3"
git push origin onr-core/v1.2.3
```

This triggers `.github/workflows/release-onr-core.yml`, which:

- verifies the tagged commit is on `main`
- runs `go test -v ./...` in `onr-core/`
- creates a GitHub release for the tag

Consumers can install it with:

```bash
go get github.com/r9s-ai/open-next-router/onr-core@v1.2.3
```

## Verify published versions

```bash
go list -m -versions github.com/r9s-ai/open-next-router/onr-core
```

## Notes

- Root tags (`v*`) and `onr-core` tags (`onr-core/v*`) are intentionally separate.
- Do not use root tags to version the `onr-core` module.
