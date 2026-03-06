"""OpenAI /v1/embeddings endpoint functions."""

from openai import OpenAI


def create_client(api_key, base_url):
    return OpenAI(api_key=api_key, base_url=base_url + "/v1")


def create_embedding(text, model, api_key, base_url):
    client = create_client(api_key=api_key, base_url=base_url)
    response = client.embeddings.create(
        model=model,
        input=text,
    )
    embedding = response.data[0].embedding
    usage = getattr(response, "usage", None)
    return {
        "object": "embedding",
        "model": model,
        "dimensions": len(embedding),
        "embedding": embedding,
        "prompt_tokens": getattr(usage, "prompt_tokens", None),
        "total_tokens": getattr(usage, "total_tokens", None),
    }
