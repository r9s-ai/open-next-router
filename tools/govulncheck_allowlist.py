#!/usr/bin/env python3
"""Reject reachable vulnerabilities."""

from __future__ import annotations

import json
import sys
from pathlib import Path
from typing import Any


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


def validate(messages: list[dict[str, Any]]) -> set[tuple[str, str, str, str]]:
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
        unexpected.add(finding_identity(finding))
    return unexpected


def main(argv: list[str]) -> int:
    if len(argv) != 2:
        print(f"usage: {argv[0]} <govulncheck-json-report>", file=sys.stderr)
        return 2
    try:
        messages = decode_messages(Path(argv[1]).read_text(encoding="utf-8"))
        unexpected = validate(messages)
    except (OSError, ValueError, json.JSONDecodeError) as exc:
        print(f"invalid govulncheck report: {exc}", file=sys.stderr)
        return 2

    if unexpected:
        print("reachable vulnerabilities:", file=sys.stderr)
        for vulnerability_id, module, version, fixed_version in sorted(unexpected):
            print(
                f"- {vulnerability_id or '<missing-id>'}: "
                f"{module or '<missing-module>'}@{version or '<missing-version>'} "
                f"(fixed in {fixed_version or '<unknown>'})",
                file=sys.stderr,
            )
        return 1

    print("govulncheck found no reachable vulnerabilities")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
