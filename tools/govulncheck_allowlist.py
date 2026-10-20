#!/usr/bin/env python3
"""Reject reachable vulnerabilities outside the narrow stdlib exception list."""

from __future__ import annotations

import json
import sys
from collections import Counter
from pathlib import Path
from typing import Any


# Temporary exception while the production toolchain remains on Go 1.26.6.
# Remove this list as soon as open-next-router can move to Go 1.26.9 or newer.
# Matching is intentionally scoped to the exact stdlib version and fixed
# version, so the same GO IDs reported for third-party modules are never
# suppressed.
ALLOWED_STDLIB_FINDINGS = {
    vulnerability_id: ("stdlib", "v1.26.6", "v1.26.9")
    for vulnerability_id in {
        "GO-2026-6599",
        "GO-2026-6600",
        "GO-2026-6603",
        "GO-2026-6605",
        "GO-2026-6607",
        "GO-2026-6608",
        "GO-2026-6610",
        "GO-2026-6611",
        "GO-2026-6612",
        "GO-2026-6613",
        "GO-2026-6617",
    }
}


def decode_messages(raw: str) -> list[dict[str, Any]]:
    decoder = json.JSONDecoder()
    messages: list[dict[str, Any]] = []
    offset = 0
    while offset < len(raw):
        while offset < len(raw) and raw[offset].isspace():
            offset += 1
        if offset == len(raw):
            break
        message, offset = decoder.raw_decode(raw, offset)
        if not isinstance(message, dict):
            raise ValueError("govulncheck message must be a JSON object")
        messages.append(message)
    return messages


def finding_identity(finding: dict[str, Any]) -> tuple[str, str, str, str]:
    trace = finding.get("trace")
    first_frame = trace[0] if isinstance(trace, list) and trace else {}
    if not isinstance(first_frame, dict):
        first_frame = {}
    return (
        str(finding.get("osv", "")),
        str(first_frame.get("module", "")),
        str(first_frame.get("version", "")),
        str(finding.get("fixed_version", "")),
    )


def validate(messages: list[dict[str, Any]]) -> tuple[Counter[str], set[tuple[str, str, str, str]]]:
    allowed: Counter[str] = Counter()
    unexpected: set[tuple[str, str, str, str]] = set()
    for message in messages:
        finding = message.get("finding")
        if not isinstance(finding, dict):
            continue
        trace = finding.get("trace")
        first_frame = trace[0] if isinstance(trace, list) and trace else {}
        # govulncheck reports module/package findings for awareness, but only
        # symbol findings (a vulnerable function is reachable) fail its normal
        # text-mode scan. Preserve that behavior here.
        if not isinstance(first_frame, dict) or not first_frame.get("function"):
            continue
        vulnerability_id, module, version, fixed_version = finding_identity(finding)
        if ALLOWED_STDLIB_FINDINGS.get(vulnerability_id) == (module, version, fixed_version):
            allowed[vulnerability_id] += 1
        else:
            unexpected.add((vulnerability_id, module, version, fixed_version))
    return allowed, unexpected


def main(argv: list[str]) -> int:
    if len(argv) != 2:
        print(f"usage: {argv[0]} <govulncheck-json-report>", file=sys.stderr)
        return 2
    try:
        messages = decode_messages(Path(argv[1]).read_text(encoding="utf-8"))
        allowed, unexpected = validate(messages)
    except (OSError, ValueError, json.JSONDecodeError) as exc:
        print(f"invalid govulncheck report: {exc}", file=sys.stderr)
        return 2

    if unexpected:
        print("unexpected reachable vulnerabilities:", file=sys.stderr)
        for vulnerability_id, module, version, fixed_version in sorted(unexpected):
            print(
                f"- {vulnerability_id or '<missing-id>'}: "
                f"{module or '<missing-module>'}@{version or '<missing-version>'} "
                f"(fixed in {fixed_version or '<unknown>'})",
                file=sys.stderr,
            )
        return 1

    if allowed:
        summary = ", ".join(f"{key} ({allowed[key]} traces)" for key in sorted(allowed))
        print(f"temporarily allowed Go 1.26.6 stdlib vulnerabilities: {summary}")
    else:
        print("govulncheck found no reachable vulnerabilities")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
