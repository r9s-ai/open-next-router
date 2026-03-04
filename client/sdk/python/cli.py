#!/usr/bin/env python3
"""
ONR Python SDK CLI
Usage:
    python cli.py openai hello --stream
    python cli.py anthropic hello --model claude-3-haiku
    python cli.py gemini hello
"""

import importlib.util
import os
import time
from pathlib import Path
from types import ModuleType

import click

ONR_API_KEY = os.environ.get("ONR_API_KEY")
ONR_BASE_URL = os.environ.get("ONR_BASE_URL", "http://localhost:3300")
ROOT = Path(__file__).resolve().parent


def _validate_environment():
    """Validate required environment variables."""
    if not ONR_API_KEY:
        raise click.ClickException(
            "Error: ONR_API_KEY environment variable is not set.\n"
            "Please set it before running the CLI:\n"
            "  export ONR_API_KEY=your-api-key"
        )


def _load_module(module_name: str, relative_path: str) -> ModuleType:
    module_path = ROOT / relative_path
    spec = importlib.util.spec_from_file_location(module_name, str(module_path))
    if spec is None or spec.loader is None:
        raise ImportError(
            f"Failed to load module spec: {module_name} from {module_path}"
        )
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def _print_verbose_metrics(
    provider,
    model,
    stream,
    elapsed_sec,
    text_chars=0,
    image_count=0,
    status="ok",
    exception_message=None,
):
    safe_elapsed = elapsed_sec if elapsed_sec > 0 else 1e-9
    tps = text_chars / safe_elapsed
    print("\n=== Request Metrics ===")
    print(f"provider: {provider}")
    print(f"model: {model}")
    print(f"base_url: {ONR_BASE_URL}")
    print(f"stream: {stream}")
    print(f"elapsed_sec: {elapsed_sec:.3f}")
    print(f"text_chars: {text_chars}")
    print(f"text_tps: {tps:.2f} chars/sec")
    print(f"status: {status}")
    if image_count > 0:
        print(f"images: {image_count}")
    if exception_message:
        print(f"exception: {exception_message}")


def _raise_sanitized_error(exc):
    if isinstance(exc, click.ClickException):
        raise exc
    raise click.ClickException(f"Request failed: {type(exc).__name__}: {exc}") from None


OPENAI_PROVIDER = _load_module("onr_openai_provider", "openai/chat_completions.py")
ANTHROPIC_PROVIDER = _load_module("onr_anthropic_provider", "anthropic/chat.py")
GEMINI_PROVIDER = _load_module("onr_gemini_provider", "gemini/gemini.py")


@click.group()
def cli():
    """ONR SDK CLI - Unified interface for OpenAI, Anthropic, Gemini"""
    pass


@cli.command()
@click.argument("prompt")
@click.option("--model", "-m", default="gpt-4o-mini", help="Model name")
@click.option("--stream/--no-stream", default=False, help="Enable streaming")
@click.option("-v", "--verbose", is_flag=True, help="Print request metrics")
def openai(prompt, model, stream, verbose):
    """OpenAI compatible API"""
    start = time.time()
    text_chars = 0
    status = "ok"
    exception_message = None
    try:
        _validate_environment()
        if stream:
            for text in OPENAI_PROVIDER.stream_chat(
                prompt, model, ONR_API_KEY, ONR_BASE_URL
            ):
                print(text, end="")
                text_chars += len(text)
            print()
            return
        response_text = OPENAI_PROVIDER.chat(prompt, model, ONR_API_KEY, ONR_BASE_URL)
        print(response_text)
        text_chars = len(response_text or "")
    except Exception as exc:
        status = "error"
        exception_message = f"{type(exc).__name__}: {exc}"
        _raise_sanitized_error(exc)
    finally:
        if verbose:
            _print_verbose_metrics(
                "openai",
                model,
                stream,
                time.time() - start,
                text_chars=text_chars,
                status=status,
                exception_message=exception_message,
            )


@cli.command()
@click.argument("prompt")
@click.option("--model", "-m", default="claude-haiku-4-5", help="Model name")
@click.option("--stream/--no-stream", default=False, help="Enable streaming")
@click.option("-v", "--verbose", is_flag=True, help="Print request metrics")
def anthropic(prompt, model, stream, verbose):
    """Anthropic API"""
    start = time.time()
    text_chars = 0
    status = "ok"
    exception_message = None
    try:
        _validate_environment()
        if stream:
            for text in ANTHROPIC_PROVIDER.stream_chat(
                prompt, model, ONR_API_KEY, ONR_BASE_URL
            ):
                print(text, end="")
                text_chars += len(text)
            print()
            return
        response_text = ANTHROPIC_PROVIDER.chat(
            prompt, model, ONR_API_KEY, ONR_BASE_URL
        )
        print(response_text)
        text_chars = len(response_text or "")
    except Exception as exc:
        status = "error"
        exception_message = f"{type(exc).__name__}: {exc}"
        _raise_sanitized_error(exc)
    finally:
        if verbose:
            _print_verbose_metrics(
                "anthropic",
                model,
                stream,
                time.time() - start,
                text_chars=text_chars,
                status=status,
                exception_message=exception_message,
            )


@cli.command()
@click.argument("prompt")
@click.option("--model", "-m", default="gemini-2.5-flash", help="Model name")
@click.option("--stream/--no-stream", default=False, help="Enable streaming")
@click.option(
    "--multimodal/--no-multimodal",
    default=False,
    help="Enable multimodal response (text+image)",
)
@click.option(
    "--image-output-dir",
    default=".",
    help="Directory to save generated images in multimodal mode",
)
@click.option("-v", "--verbose", is_flag=True, help="Print request metrics")
def gemini(prompt, model, stream, multimodal, image_output_dir, verbose):
    """Google Gemini API"""
    start = time.time()
    text_chars = 0
    image_count = 0
    status = "ok"
    exception_message = None
    try:
        _validate_environment()
        if multimodal and not stream:
            raise click.ClickException("--multimodal requires --stream")

        if multimodal:
            output_dir = Path(image_output_dir)
            output_dir.mkdir(parents=True, exist_ok=True)
            for event in GEMINI_PROVIDER.stream_chat_multimodal(
                prompt,
                model,
                ONR_API_KEY,
                ONR_BASE_URL,
                ["TEXT", "IMAGE"],
            ):
                if event["type"] == "text":
                    print(event["text"], end="")
                    text_chars += len(event["text"])
                    continue
                if event["type"] == "image":
                    image_count += 1
                    image_path = output_dir / f"gemini_{image_count}.png"
                    event["image"].save(str(image_path))
                    print(f"\n[image saved] {image_path}")
            print()
            return

        if stream:
            for text in GEMINI_PROVIDER.stream_chat(
                prompt, model, ONR_API_KEY, ONR_BASE_URL
            ):
                print(text, end="")
                text_chars += len(text)
            print()
            return

        response_text = GEMINI_PROVIDER.chat(prompt, model, ONR_API_KEY, ONR_BASE_URL)
        print(response_text)
        text_chars = len(response_text or "")
    except Exception as exc:
        status = "error"
        exception_message = f"{type(exc).__name__}: {exc}"
        _raise_sanitized_error(exc)
    finally:
        if verbose:
            _print_verbose_metrics(
                "gemini",
                model,
                stream,
                time.time() - start,
                text_chars=text_chars,
                image_count=image_count,
                status=status,
                exception_message=exception_message,
            )


if __name__ == "__main__":
    cli()
