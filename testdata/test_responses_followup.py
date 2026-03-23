import json
import urllib.request

prev_id = "resp_984423b8bcd6c145cc161699c49ab7412f1533d284448719"

body = {
    "model": "qwen3.5-27b@q4_k_s",
    "input": "What was the name of the first function in that file? Answer with just the function name.",
    "previous_response_id": prev_id
}

payload = json.dumps(body).encode("utf-8")
print(f"Testing stateful follow-up with previous_response_id: {prev_id}")

req = urllib.request.Request(
    "http://localhost:1234/v1/responses",
    data=payload,
    headers={"Content-Type": "application/json", "Authorization": "Bearer sk-lm-K52VcdOs:8ewvJcZ0vK9gaEILPghx"},
    method="POST"
)
try:
    with urllib.request.urlopen(req, timeout=300) as resp:
        data = json.loads(resp.read().decode("utf-8"))
        print(f"Response ID: {data.get('id', '(none)')}")
        if "output" in data:
            for item in data["output"]:
                if item.get("type") == "message":
                    for c in item.get("content", []):
                        if c.get("type") == "output_text":
                            print(f"Model answer: {c.get('text', '(no text)')}")
            print(f"Status: {data.get('status')}")
            usage = data.get("usage", {})
            print(f"Tokens: input={usage.get('input_tokens')}, output={usage.get('output_tokens')}")
        else:
            print(f"Full response: {json.dumps(data, indent=2)[:2000]}")
except urllib.error.HTTPError as e:
    body = e.read().decode("utf-8")
    print(f"HTTP {e.code}: {body[:2000]}")
