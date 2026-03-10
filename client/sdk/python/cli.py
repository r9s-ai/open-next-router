#!/usr/bin/env python3
"""
ONR Python SDK CLI
Usage:
    python cli.py openai chat_completions hello --stream
    python cli.py openai responses hello --stream
    python cli.py openai embeddings "hello embeddings"
    python cli.py anthropic messages hello --model claude-3-haiku
    python cli.py gemini models hello
    python cli.py gemini chats "draw a guitar"
"""

import importlib.util
import os
import subprocess
import sys
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


OPENAI_CHAT_PROVIDER = _load_module(
    "onr_openai_chat_provider", "openai/chat_completions.py"
)
OPENAI_EMBEDDINGS_PROVIDER = _load_module(
    "onr_openai_embeddings_provider", "openai/embeddings.py"
)
OPENAI_RESPONSES_PROVIDER = _load_module(
    "onr_openai_responses_provider", "openai/responses.py"
)
ANTHROPIC_PROVIDER = _load_module("onr_anthropic_provider", "anthropic/messages.py")
GEMINI_MODELS_PROVIDER = _load_module("onr_gemini_models_provider", "gemini/models.py")


@click.group()
def cli():
    """ONR SDK CLI - Unified interface for OpenAI, Anthropic, Gemini"""
    pass


@cli.command("completion")
@click.option(
    "--shell",
    type=click.Choice(["bash", "zsh", "fish"], case_sensitive=False),
    required=True,
    help="Shell type",
)
def completion(shell):
    """Print shell completion script."""
    shell = shell.lower()
    complete_env = "_CLI_PY_COMPLETE"
    mode = f"{shell}_source"
    env = dict(os.environ)
    env[complete_env] = mode
    result = subprocess.run(
        [sys.executable, str(ROOT / "cli.py")],
        env=env,
        capture_output=True,
        text=True,
        check=False,
    )
    if result.returncode != 0:
        stderr = result.stderr.strip() or "failed to generate completion script"
        raise click.ClickException(stderr)
    print(result.stdout, end="")


@cli.group()
def openai():
    """OpenAI API"""
    pass


@openai.command("chat_completions")
@click.argument("prompt")
@click.option("--model", "-m", default="gpt-4o-mini", help="Model name")
@click.option("--stream/--no-stream", default=False, help="Enable streaming")
@click.option("-v", "--verbose", is_flag=True, help="Print request metrics")
def openai_chat_completions(prompt, model, stream, verbose):
    """OpenAI chat completions"""
    start = time.time()
    text_chars = 0
    status = "ok"
    exception_message = None
    try:
        _validate_environment()
        if stream:
            for text in OPENAI_CHAT_PROVIDER.stream_chat(
                prompt, model, ONR_API_KEY, ONR_BASE_URL
            ):
                print(text, end="")
                text_chars += len(text)
            print()
            return
        response_text = OPENAI_CHAT_PROVIDER.chat(
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
                "openai/chat_completions",
                model,
                stream,
                time.time() - start,
                text_chars=text_chars,
                status=status,
                exception_message=exception_message,
            )


@openai.command("embeddings")
@click.argument("text")
@click.option("--model", "-m", default="text-embedding-3-small", help="Model name")
@click.option("-v", "--verbose", is_flag=True, help="Print request metrics")
def openai_embeddings(text, model, verbose):
    """OpenAI embeddings"""
    start = time.time()
    text_chars = 0
    status = "ok"
    exception_message = None
    try:
        _validate_environment()
        embedding_resp = OPENAI_EMBEDDINGS_PROVIDER.create_embedding(
            text, model, ONR_API_KEY, ONR_BASE_URL
        )
        dimensions = embedding_resp["dimensions"]
        preview = embedding_resp["embedding"][:8]
        print(f"object: {embedding_resp['object']}")
        print(f"model: {embedding_resp['model']}")
        print(f"dimensions: {dimensions}")
        print(f"prompt_tokens: {embedding_resp['prompt_tokens']}")
        print(f"total_tokens: {embedding_resp['total_tokens']}")
        print(f"embedding_preview: {preview}")
        text_chars = len(str(preview))
    except Exception as exc:
        status = "error"
        exception_message = f"{type(exc).__name__}: {exc}"
        _raise_sanitized_error(exc)
    finally:
        if verbose:
            _print_verbose_metrics(
                "openai/embeddings",
                model,
                False,
                time.time() - start,
                text_chars=text_chars,
                status=status,
                exception_message=exception_message,
            )


@openai.command("responses")
@click.argument("prompt")
@click.option("--model", "-m", default="gpt-4o-mini", help="Model name")
@click.option("--stream/--no-stream", default=False, help="Enable streaming")
@click.option("-v", "--verbose", is_flag=True, help="Print request metrics")
def openai_responses(prompt, model, stream, verbose):
    """OpenAI responses"""
    start = time.time()
    text_chars = 0
    status = "ok"
    exception_message = None
    try:
        _validate_environment()
        if stream:
            for text in OPENAI_RESPONSES_PROVIDER.stream_response(
                prompt, model, ONR_API_KEY, ONR_BASE_URL
            ):
                print(text, end="")
                text_chars += len(text)
            print()
            return
        response_text = OPENAI_RESPONSES_PROVIDER.create_response(
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
                "openai/responses",
                model,
                stream,
                time.time() - start,
                text_chars=text_chars,
                status=status,
                exception_message=exception_message,
            )


@cli.group()
def anthropic():
    """Anthropic API"""
    pass


@anthropic.command("messages")
@click.argument("prompt")
@click.option("--model", "-m", default="claude-haiku-4-5", help="Model name")
@click.option("--stream/--no-stream", default=False, help="Enable streaming")
@click.option("-v", "--verbose", is_flag=True, help="Print request metrics")
def anthropic_messages(prompt, model, stream, verbose):
    """Anthropic messages"""
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
                "anthropic/messages",
                model,
                stream,
                time.time() - start,
                text_chars=text_chars,
                status=status,
                exception_message=exception_message,
            )


@cli.group()
def gemini():
    """Google Gemini API"""
    pass


@gemini.command("models")
@click.argument("prompt")
@click.option("--model", "-m", default="gemini-2.5-flash", help="Model name")
@click.option("--stream/--no-stream", default=False, help="Enable streaming")
@click.option("-v", "--verbose", is_flag=True, help="Print request metrics")
def gemini_models(prompt, model, stream, verbose):
    """Gemini models.generate_content"""
    start = time.time()
    text_chars = 0
    status = "ok"
    exception_message = None
    try:
        _validate_environment()

        if stream:
            for text in GEMINI_MODELS_PROVIDER.stream_chat(
                prompt, model, ONR_API_KEY, ONR_BASE_URL
            ):
                print(text, end="")
                text_chars += len(text)
            print()
            return

        response_text = GEMINI_MODELS_PROVIDER.chat(
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
                "gemini/models",
                model,
                stream,
                time.time() - start,
                text_chars=text_chars,
                status=status,
                exception_message=exception_message,
            )


@gemini.command("chats")
@click.argument("prompt")
@click.option("--model", "-m", default="gemini-3-pro-image-preview", help="Model name")
@click.option(
    "--response_modalities",
    default="TEXT,IMAGE",
    help="Comma-separated response modalities. Example: --response_modalities TEXT,IMAGE",
)
@click.option(
    "--image-output-dir",
    default=".",
    help="Directory to save generated images",
)
@click.option("-v", "--verbose", is_flag=True, help="Print request metrics")
def gemini_chats(prompt, model, response_modalities, image_output_dir, verbose):
    """Gemini chats.send_message_stream"""
    start = time.time()
    text_chars = 0
    image_count = 0
    status = "ok"
    exception_message = None
    try:
        _validate_environment()
        output_dir = Path(image_output_dir)
        output_dir.mkdir(parents=True, exist_ok=True)
        modalities = [item.strip() for item in response_modalities.split(",") if item.strip()]
        for event in GEMINI_MODELS_PROVIDER.stream_chat_multimodal(
            prompt,
            model,
            ONR_API_KEY,
            ONR_BASE_URL,
            modalities,
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
    except Exception as exc:
        status = "error"
        exception_message = f"{type(exc).__name__}: {exc}"
        _raise_sanitized_error(exc)
    finally:
        if verbose:
            _print_verbose_metrics(
                "gemini/chats",
                model,
                True,
                time.time() - start,
                text_chars=text_chars,
                image_count=image_count,
                status=status,
                exception_message=exception_message,
            )


if __name__ == "__main__":
    cli()
