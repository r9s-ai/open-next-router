"""Anthropic /v1/messages endpoint functions."""

from anthropic import Anthropic


def create_client(api_key, base_url):
    return Anthropic(api_key=api_key, base_url=base_url)


def chat(prompt, model, api_key, base_url, max_tokens=1024):
    client = create_client(api_key=api_key, base_url=base_url)
    response = client.messages.create(
        model=model,
        max_tokens=max_tokens,
        messages=[{"role": "user", "content": prompt}],
    )
    return response.content[0].text


def stream_chat(prompt, model, api_key, base_url, max_tokens=1024):
    client = create_client(api_key=api_key, base_url=base_url)
    with client.messages.stream(
        model=model,
        max_tokens=max_tokens,
        messages=[{"role": "user", "content": prompt}],
    ) as stream:
        yield from stream.text_stream
