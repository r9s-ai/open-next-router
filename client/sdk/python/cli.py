"""
ONR Python SDK CLI
Usage:
    python cli.py openai hello --stream
    python cli.py anthropic hello --model claude-3-haiku
    python cli.py gemini hello
"""
import os
import click
import importlib.util
from pathlib import Path


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


def _load_module(module_name, relative_path):
    module_path = ROOT / relative_path
    spec = importlib.util.spec_from_file_location(module_name, str(module_path))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


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
def openai(prompt, model, stream):
    """OpenAI compatible API"""
    _validate_environment()
    if stream:
        for text in OPENAI_PROVIDER.stream_chat(prompt, model, ONR_API_KEY, ONR_BASE_URL):
            print(text, end="")
        print()
        return
    print(OPENAI_PROVIDER.chat(prompt, model, ONR_API_KEY, ONR_BASE_URL))


@cli.command()
@click.argument("prompt")
@click.option("--model", "-m", default="claude-haiku-4-5", help="Model name")
@click.option("--stream/--no-stream", default=False, help="Enable streaming")
def anthropic(prompt, model, stream):
    """Anthropic API"""
    _validate_environment()
    if stream:
        for text in ANTHROPIC_PROVIDER.stream_chat(prompt, model, ONR_API_KEY, ONR_BASE_URL):
            print(text, end="")
        print()
        return
    print(ANTHROPIC_PROVIDER.chat(prompt, model, ONR_API_KEY, ONR_BASE_URL))


@cli.command()
@click.argument("prompt")
@click.option("--model", "-m", default="gemini-2.5-flash", help="Model name")
@click.option("--stream/--no-stream", default=False, help="Enable streaming")
def gemini(prompt, model, stream):
    """Google Gemini API"""
    _validate_environment()
    if stream:
        for text in GEMINI_PROVIDER.stream_chat(prompt, model, ONR_API_KEY, ONR_BASE_URL):
            print(text, end="")
        print()
        return
    print(GEMINI_PROVIDER.chat(prompt, model, ONR_API_KEY, ONR_BASE_URL))


if __name__ == "__main__":
    cli()
