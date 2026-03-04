"""Gemini provider functions."""

from google import genai
from google.genai import types


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


def stream_chat_multimodal(prompt, model, api_key, base_url, response_modalities=None):
    client = create_client(api_key=api_key, base_url=base_url)
    modalities = response_modalities or ["TEXT", "IMAGE"]
    chat = client.chats.create(
        model=model,
        config=types.GenerateContentConfig(response_modalities=modalities),
    )

    response = chat.send_message_stream(prompt)
    for chunk in response:
        if not chunk.candidates:
            continue
        candidate = chunk.candidates[0]
        if not candidate.content or not candidate.content.parts:
            continue

        for part in candidate.content.parts:
            if part.text is not None:
                yield {"type": "text", "text": part.text}
                continue

            image = part.as_image()
            if image is not None:
                yield {"type": "image", "image": image}
