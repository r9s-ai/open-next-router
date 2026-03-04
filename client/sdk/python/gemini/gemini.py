"""Gemini provider functions."""

from google import genai


def create_client(api_key, base_url):
    """Create Gemini client with API key and base URL."""
    return genai.Client(api_key=api_key, http_options={"base_url": base_url})


def chat(prompt, model, api_key, base_url):
    client = create_client(api_key=api_key, base_url=base_url)
    response = client.models.generate_content(
        model=model,
        contents=prompt,
    )
    return response.text


def stream_chat(prompt, model, api_key, base_url):
    client = create_client(api_key=api_key, base_url=base_url)
    response = client.models.generate_content_stream(
        model=model,
        contents=prompt,
    )
    for chunk in response:
        if hasattr(chunk, 'text') and chunk.text:
            yield chunk.text
