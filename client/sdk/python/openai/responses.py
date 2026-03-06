"""OpenAI /v1/responses endpoint functions."""

from openai import OpenAI


def create_client(api_key, base_url):
    return OpenAI(api_key=api_key, base_url=base_url + "/v1")


def create_response(prompt, model, api_key, base_url):
    client = create_client(api_key=api_key, base_url=base_url)
    response = client.responses.create(
        model=model,
        input=prompt,
    )
    output_text = getattr(response, "output_text", None)
    if output_text:
        return output_text

    texts = []
    for item in getattr(response, "output", []) or []:
        for content in getattr(item, "content", []) or []:
            text = getattr(content, "text", None)
            if text:
                texts.append(text)
    return "".join(texts)


def stream_response(prompt, model, api_key, base_url):
    client = create_client(api_key=api_key, base_url=base_url)
    stream = client.responses.create(
        model=model,
        input=prompt,
        stream=True,
    )
    for event in stream:
        if getattr(event, "type", "") == "response.output_text.delta":
            delta = getattr(event, "delta", None)
            if delta:
                yield delta
