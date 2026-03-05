# vai2oai

`vai2oai` is a small HTTP proxy for a specific LiteLLM SDK behavior:

- LiteLLM SDK may infer provider routing from model names.
- If your model name includes `vertex_ai`, the SDK can try Vertex AI auth locally and ask for Google credentials.
- In some setups, you do not want local Vertex AI auth at all. You only want to call your custom endpoint.

This proxy keeps the client-side model name provider-neutral, then rewrites it on the way upstream.

## How it works

1. Your client sends requests to `vai2oai`.
2. `vai2oai` forwards every path and method to `UPSTREAM_BASE_URL`.
3. If request body is JSON and has a top-level string `model`, it rewrites:
   - `gemini-2.0-flash` -> `vertex_ai/gemini-2.0-flash`
4. Upstream receives the provider-prefixed model, but your client does not need to send `vertex_ai/...` directly.

This avoids triggering local Vertex AI credential checks in SDK flows where `vertex_ai` in the client model name causes trouble.

## Environment variables

- `UPSTREAM_BASE_URL` (required): upstream base URL, for example `https://your-litellm-gateway`
- `UPSTREAM_API_KEY` (optional): if set, proxy overwrites `Authorization` with `Bearer <UPSTREAM_API_KEY>`
- `PORT` (optional): listen port, default `8080`

A `.env` file is supported via `godotenv`.

## Run locally

```bash
go run .
```

Or build first:

```bash
go build -o vai2oai .
./vai2oai
```

## Docker

```bash
docker build -t vai2oai .
docker run --rm -p 8080:8080 \
  -e UPSTREAM_BASE_URL=https://your-litellm-gateway \
  -e UPSTREAM_API_KEY=your-key \
  vai2oai
```

## Client example

Point your SDK/client to this proxy instead of the upstream endpoint.

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:8080/v1",
    api_key="dummy"
)

resp = client.chat.completions.create(
    model="gemini-2.0-flash",  # no vertex_ai prefix here
    messages=[{"role": "user", "content": "Hello"}],
)
```

The proxy forwards to upstream with `model="vertex_ai/gemini-2.0-flash"`.

## Notes

- Rewriting only applies when request body is valid JSON with top-level string field `model`.
- If `model` already starts with `vertex_ai/`, it is not changed.
- Non-JSON bodies are passed through unchanged.
- Responses are streamed through as-is (works with SSE/chat streaming).
