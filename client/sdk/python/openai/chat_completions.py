"""OpenAI provider functions."""

from openai import OpenAI


def create_client(api_key, base_url):
    return OpenAI(api_key=api_key, base_url=base_url+"/v1")


def chat(prompt, model, api_key, base_url):
    client = create_client(api_key=api_key, base_url=base_url)
    response = client.chat.completions.create(
        model=model,
        messages=[{"role": "user", "content": prompt}],
    )
    return response.choices[0].message.content


def stream_chat(prompt, model, api_key, base_url):
    client = create_client(api_key=api_key, base_url=base_url)
    stream = client.chat.completions.create(
        model=model,
        messages=[{"role": "user", "content": prompt}],
        stream=True,
    )
    for chunk in stream:
        if chunk.choices and len(chunk.choices) > 0:
            delta = chunk.choices[0].delta.content
            if delta:
                yield delta
