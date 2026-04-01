#!/usr/bin/env python3
"""Render ONR provider config (*.conf) from a JSON spec."""

from __future__ import annotations

import argparse
import json
import re
import sys
from copy import deepcopy
from pathlib import Path
from typing import Any, Dict, Iterable, List

VALID_APIS = {
    "completions",
    "chat.completions",
    "responses",
    "claude.messages",
    "embeddings",
    "images.generations",
    "audio.speech",
    "audio.transcriptions",
    "audio.translations",
    "gemini.generateContent",
    "gemini.streamGenerateContent",
}

PROVIDER_RE = re.compile(r"^[a-z0-9][a-z0-9-]{0,63}$")


def fail(message: str) -> None:
    print(f"[ERROR] {message}", file=sys.stderr)
    raise SystemExit(1)


def quote(value: str) -> str:
    escaped = value.replace("\\", "\\\\").replace('"', '\\"')
    return f'"{escaped}"'


def format_expr(value: Any) -> str:
    if isinstance(value, dict):
        if set(value.keys()) != {"expr"} or not isinstance(value["expr"], str):
            fail("expression object must be exactly: {\"expr\": \"...\"}")
        return value["expr"]
    if isinstance(value, bool):
        return "true" if value else "false"
    if isinstance(value, (int, float)):
        return str(value)
    if isinstance(value, str):
        return quote(value)
    fail(f"unsupported expression value type: {type(value).__name__}")


def ensure_list(value: Any, field: str) -> List[Any]:
    if value is None:
        return []
    if not isinstance(value, list):
        fail(f"{field} must be a list")
    return value


def append_raw(lines: List[str], raw: Iterable[str]) -> None:
    for item in raw:
        if not isinstance(item, str) or not item.strip():
            fail("raw directive must be a non-empty string")
        stmt = item.strip()
        lines.append(stmt if stmt.endswith(";") else stmt + ";")


def render_auth(auth_cfg: Dict[str, Any]) -> List[str]:
    mode = auth_cfg.get("mode", "bearer")
    lines: List[str] = []

    if mode == "bearer":
        lines.append("auth_bearer;")
    elif mode == "header":
        header = auth_cfg.get("header")
        if not isinstance(header, str) or not header.strip():
            fail("auth.header is required when auth.mode=header")
        lines.append(f"auth_header_key {quote(header.strip())};")
    elif mode == "oauth":
        oauth_mode = auth_cfg.get("oauth_mode")
        if not isinstance(oauth_mode, str) or not oauth_mode.strip():
            fail("auth.oauth_mode is required when auth.mode=oauth")
        lines.append(f"oauth_mode {oauth_mode.strip()};")

        refresh_expr = auth_cfg.get("oauth_refresh_token", {"expr": "$channel.key"})
        lines.append(f"oauth_refresh_token {format_expr(refresh_expr)};")

        optional_expr_fields = {
            "oauth_token_url": "oauth_token_url",
            "oauth_client_id": "oauth_client_id",
            "oauth_client_secret": "oauth_client_secret",
            "oauth_scope": "oauth_scope",
            "oauth_audience": "oauth_audience",
            "oauth_token_path": "oauth_token_path",
            "oauth_expires_in_path": "oauth_expires_in_path",
            "oauth_token_type_path": "oauth_token_type_path",
        }
        for field, directive in optional_expr_fields.items():
            if field in auth_cfg:
                lines.append(f"{directive} {format_expr(auth_cfg[field])};")

        optional_scalar_fields = {
            "oauth_method": "oauth_method",
            "oauth_content_type": "oauth_content_type",
            "oauth_timeout_ms": "oauth_timeout_ms",
            "oauth_refresh_skew_sec": "oauth_refresh_skew_sec",
            "oauth_fallback_ttl_sec": "oauth_fallback_ttl_sec",
        }
        for field, directive in optional_scalar_fields.items():
            if field in auth_cfg:
                lines.append(f"{directive} {auth_cfg[field]};")

        oauth_forms = ensure_list(auth_cfg.get("oauth_form"), "auth.oauth_form")
        for form_item in oauth_forms:
            if not isinstance(form_item, dict):
                fail("auth.oauth_form items must be objects")
            key = form_item.get("key")
            if not isinstance(key, str) or not key.strip():
                fail("auth.oauth_form[].key is required")
            if "value" not in form_item:
                fail("auth.oauth_form[].value is required")
            lines.append(f"oauth_form {quote(key.strip())} {format_expr(form_item['value'])};")

        lines.append("auth_oauth_bearer;")
    else:
        fail(f"unsupported auth.mode: {mode}")

    append_raw(lines, ensure_list(auth_cfg.get("extra_directives"), "auth.extra_directives"))
    return lines


def render_metrics(metrics_cfg: Dict[str, Any]) -> List[str]:
    lines: List[str] = []
    usage_mode = metrics_cfg.get("usage_extract")
    finish_mode = metrics_cfg.get("finish_reason_extract")

    if usage_mode:
        lines.append(f"usage_extract {usage_mode};")
    if finish_mode:
        lines.append(f"finish_reason_extract {finish_mode};")

    if "finish_reason_path" in metrics_cfg:
        lines.append(f"finish_reason_path {format_expr(metrics_cfg['finish_reason_path'])};")

    token_keys = [
        "input_tokens",
        "output_tokens",
        "cache_read_tokens",
        "cache_write_tokens",
        "total_tokens",
    ]
    for key in token_keys:
        if key in metrics_cfg:
            lines.append(f"{key} = {format_expr(metrics_cfg[key])};")
        path_key = f"{key}_path"
        if path_key in metrics_cfg:
            lines.append(f"{path_key} {format_expr(metrics_cfg[path_key])};")

    append_raw(lines, ensure_list(metrics_cfg.get("extra_directives"), "metrics.extra_directives"))
    return lines


def render_models(models_cfg: Dict[str, Any]) -> List[str]:
    mode = models_cfg.get("mode")
    if not isinstance(mode, str) or not mode.strip():
        fail("models.mode is required")

    lines: List[str] = [f"models_mode {mode.strip()};"]

    if "path" in models_cfg:
        lines.append(f"path {quote(str(models_cfg['path']))};")
    if "method" in models_cfg:
        lines.append(f"method {models_cfg['method']};")

    for id_path in ensure_list(models_cfg.get("id_path"), "models.id_path"):
        if not isinstance(id_path, str) or not id_path.strip():
            fail("models.id_path must contain non-empty strings")
        lines.append(f"id_path {quote(id_path)};")

    if "id_regex" in models_cfg:
        lines.append(f"id_regex {quote(str(models_cfg['id_regex']))};")
    if "id_allow_regex" in models_cfg:
        lines.append(f"id_allow_regex {quote(str(models_cfg['id_allow_regex']))};")

    for name, value in (models_cfg.get("set_headers") or {}).items():
        lines.append(f"set_header {quote(str(name))} {format_expr(value)};")
    for name in ensure_list(models_cfg.get("del_headers"), "models.del_headers"):
        lines.append(f"del_header {quote(str(name))};")

    append_raw(lines, ensure_list(models_cfg.get("extra_directives"), "models.extra_directives"))
    return lines


def render_balance(balance_cfg: Dict[str, Any]) -> List[str]:
    mode = balance_cfg.get("mode")
    if not isinstance(mode, str) or not mode.strip():
        fail("balance.mode is required")

    lines: List[str] = [f"balance_mode {mode.strip()};"]

    if "method" in balance_cfg:
        lines.append(f"method {balance_cfg['method']};")
    if "path" in balance_cfg:
        lines.append(f"path {quote(str(balance_cfg['path']))};")
    if "balance_expr" in balance_cfg:
        lines.append(f"balance_expr = {format_expr(balance_cfg['balance_expr'])};")
    if "balance_path" in balance_cfg:
        lines.append(f"balance_path {format_expr(balance_cfg['balance_path'])};")
    if "used_expr" in balance_cfg:
        lines.append(f"used_expr = {format_expr(balance_cfg['used_expr'])};")
    if "used_path" in balance_cfg:
        lines.append(f"used_path {format_expr(balance_cfg['used_path'])};")
    if "balance_unit" in balance_cfg:
        lines.append(f"balance_unit {balance_cfg['balance_unit']};")
    if "subscription_path" in balance_cfg:
        lines.append(f"subscription_path {quote(str(balance_cfg['subscription_path']))};")
    if "usage_path" in balance_cfg:
        lines.append(f"usage_path {quote(str(balance_cfg['usage_path']))};")

    for name, value in (balance_cfg.get("set_headers") or {}).items():
        lines.append(f"set_header {quote(str(name))} {format_expr(value)};")
    for name in ensure_list(balance_cfg.get("del_headers"), "balance.del_headers"):
        lines.append(f"del_header {quote(str(name))};")

    append_raw(lines, ensure_list(balance_cfg.get("extra_directives"), "balance.extra_directives"))
    return lines


def render_block(name: str, lines: List[str], level: int = 2) -> List[str]:
    indent = " " * level
    block = [f"{indent}{name} {{"]
    for stmt in lines:
        block.append(f"{indent}  {stmt}")
    block.append(f"{indent}}}")
    return block


def render_request_from_map(request_cfg: Dict[str, Any]) -> List[str]:
    lines: List[str] = []

    for header, value in (request_cfg.get("set_headers") or {}).items():
        lines.append(f"set_header {quote(str(header))} {format_expr(value)};")
    for header in ensure_list(request_cfg.get("del_headers"), "request.del_headers"):
        lines.append(f"del_header {quote(str(header))};")

    for json_path, value in (request_cfg.get("json_set") or {}).items():
        lines.append(f"json_set {quote(str(json_path))} {format_expr(value)};")
    for json_path in ensure_list(request_cfg.get("json_del"), "request.json_del"):
        lines.append(f"json_del {quote(str(json_path))};")

    for rename in ensure_list(request_cfg.get("json_rename"), "request.json_rename"):
        if not isinstance(rename, dict):
            fail("request.json_rename entries must be objects")
        src = rename.get("from")
        dst = rename.get("to")
        if not isinstance(src, str) or not isinstance(dst, str):
            fail("request.json_rename entries require string fields: from, to")
        lines.append(f"json_rename {quote(src)} {quote(dst)};")

    append_raw(lines, ensure_list(request_cfg.get("extra_directives"), "request.extra_directives"))
    return lines


def render_response_from_map(response_cfg: Dict[str, Any]) -> List[str]:
    lines: List[str] = []
    mode = response_cfg.get("mode")
    if mode == "passthrough":
        lines.append("resp_passthrough;")

    if "resp_map" in response_cfg:
        lines.append(f"resp_map {response_cfg['resp_map']};")
    if "sse_parse" in response_cfg:
        lines.append(f"sse_parse {response_cfg['sse_parse']};")

    append_raw(lines, ensure_list(response_cfg.get("extra_directives"), "response.extra_directives"))
    return lines


def render_upstream_route(route_cfg: Dict[str, Any]) -> List[str]:
    lines: List[str] = []

    if "path_expr" in route_cfg:
        lines.append(f"set_path {route_cfg['path_expr']};")
    elif "path" in route_cfg:
        lines.append(f"set_path {quote(str(route_cfg['path']))};")
    else:
        fail("route requires path or path_expr")

    for name, value in (route_cfg.get("set_query") or {}).items():
        lines.append(f"set_query {quote(str(name))} {format_expr(value)};")

    for name in ensure_list(route_cfg.get("del_query"), "route.del_query"):
        lines.append(f"del_query {quote(str(name))};")

    append_raw(lines, ensure_list(route_cfg.get("upstream_extra_directives"), "route.upstream_extra_directives"))
    return lines


def apply_preset(spec: Dict[str, Any]) -> Dict[str, Any]:
    spec = deepcopy(spec)
    preset = spec.get("preset")
    if preset is None:
        return spec

    if preset != "openai-compatible":
        fail(f"unsupported preset: {preset}")

    spec.setdefault("auth", {"mode": "bearer"})
    spec.setdefault(
        "metrics",
        {
            "usage_extract": "openai",
            "finish_reason_extract": "openai",
        },
    )
    spec.setdefault("response", {"mode": "passthrough"})
    spec.setdefault("models", {"mode": "openai"})

    if "routes" not in spec:
        spec["routes"] = [
            {"api": "chat.completions", "path": "/v1/chat/completions"},
            {"api": "completions", "path": "/v1/completions"},
            {"api": "responses", "path": "/v1/responses"},
            {"api": "embeddings", "path": "/v1/embeddings"},
        ]

    return spec


def render_provider_conf(spec_raw: Dict[str, Any]) -> str:
    spec = apply_preset(spec_raw)

    provider = spec.get("provider")
    if not isinstance(provider, str) or not provider.strip():
        fail("provider is required")
    provider = provider.strip().lower()
    if not PROVIDER_RE.match(provider):
        fail("provider must match ^[a-z0-9][a-z0-9-]{0,63}$")

    base_url = spec.get("base_url")
    if not isinstance(base_url, str) or not base_url.strip():
        fail("base_url is required")

    routes = ensure_list(spec.get("routes"), "routes")
    if not routes:
        fail("at least one route is required")

    lines: List[str] = ['syntax "next-router/0.1";', "", f"provider {quote(provider)} {{", "  defaults {"]

    lines.extend(render_block("upstream_config", [f"base_url = {quote(base_url)};"], level=4))

    auth_cfg = spec.get("auth", {"mode": "bearer"})
    if not isinstance(auth_cfg, dict):
        fail("auth must be an object")
    lines.extend(render_block("auth", render_auth(auth_cfg), level=4))

    if isinstance(spec.get("request"), dict):
        req_lines = render_request_from_map(spec["request"])
        if req_lines:
            lines.extend(render_block("request", req_lines, level=4))

    response_cfg = spec.get("response")
    if response_cfg is None:
        response_cfg = {"mode": "passthrough"}
    if not isinstance(response_cfg, dict):
        fail("response must be an object")
    response_lines = render_response_from_map(response_cfg)
    if response_lines:
        lines.extend(render_block("response", response_lines, level=4))

    metrics_cfg = spec.get("metrics")
    if isinstance(metrics_cfg, dict):
        metrics_lines = render_metrics(metrics_cfg)
        if metrics_lines:
            lines.extend(render_block("metrics", metrics_lines, level=4))

    error_map = spec.get("error_map")
    if error_map:
        lines.extend(render_block("error", [f"error_map {error_map};"], level=4))

    if isinstance(spec.get("balance"), dict):
        lines.extend(render_block("balance", render_balance(spec["balance"]), level=4))

    if isinstance(spec.get("models"), dict):
        lines.extend(render_block("models", render_models(spec["models"]), level=4))

    lines.append("  }")

    for route in routes:
        if not isinstance(route, dict):
            fail("each route must be an object")

        api = route.get("api")
        if not isinstance(api, str) or api not in VALID_APIS:
            fail(f"route.api must be one of supported api names, got: {api!r}")

        match_expr = f'  match api = {quote(api)}'
        if "stream" in route:
            stream = route["stream"]
            if not isinstance(stream, bool):
                fail("route.stream must be true/false when provided")
            match_expr += f" stream = {'true' if stream else 'false'}"
        match_expr += " {"
        lines.append("")
        lines.append(match_expr)

        route_auth = route.get("auth")
        if isinstance(route_auth, dict):
            lines.extend(render_block("auth", render_auth(route_auth), level=4))

        route_request = route.get("request")
        if isinstance(route_request, dict):
            req_lines = render_request_from_map(route_request)
            if req_lines:
                lines.extend(render_block("request", req_lines, level=4))

        lines.extend(render_block("upstream", render_upstream_route(route), level=4))

        route_response = route.get("response")
        if isinstance(route_response, dict):
            response_lines = render_response_from_map(route_response)
            if response_lines:
                lines.extend(render_block("response", response_lines, level=4))

        route_metrics = route.get("metrics")
        if isinstance(route_metrics, dict):
            metric_lines = render_metrics(route_metrics)
            if metric_lines:
                lines.extend(render_block("metrics", metric_lines, level=4))

        route_error_map = route.get("error_map")
        if route_error_map:
            lines.extend(render_block("error", [f"error_map {route_error_map};"], level=4))

        route_extra: List[str] = []
        append_raw(route_extra, ensure_list(route.get("extra_directives"), "route.extra_directives"))
        lines.extend([f"    {stmt}" for stmt in route_extra])

        lines.append("  }")

    lines.append("}")

    return "\n".join(lines) + "\n"


def main() -> None:
    parser = argparse.ArgumentParser(description="Render ONR provider config from JSON spec")
    parser.add_argument("--spec", required=True, help="Path to JSON spec file")
    parser.add_argument("--output-dir", default="config/providers", help="Output directory (default: config/providers)")
    parser.add_argument("--overwrite", action="store_true", help="Overwrite existing file")
    parser.add_argument("--stdout", action="store_true", help="Print config to stdout only")
    args = parser.parse_args()

    spec_path = Path(args.spec).resolve()
    if not spec_path.exists() or not spec_path.is_file():
        fail(f"spec file not found: {spec_path}")

    try:
        spec = json.loads(spec_path.read_text())
    except json.JSONDecodeError as exc:
        fail(f"invalid JSON spec: {exc}")

    conf_text = render_provider_conf(spec)

    if args.stdout:
        sys.stdout.write(conf_text)
        return

    provider = str(spec.get("provider", "")).strip().lower()
    if not provider:
        fail("provider is required in spec")

    out_dir = Path(args.output_dir).resolve()
    out_dir.mkdir(parents=True, exist_ok=True)
    out_file = out_dir / f"{provider}.conf"

    if out_file.exists() and not args.overwrite:
        fail(f"output file exists: {out_file} (use --overwrite)")

    out_file.write_text(conf_text)
    print(f"[OK] Wrote provider config: {out_file}")


if __name__ == "__main__":
    main()
