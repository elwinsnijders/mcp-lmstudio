import json
import urllib.request

with open(r"C:\Users\Elwin\projects\mcp-lmstudio\testdata\large_test.go", "r") as f:
    content = f.read()

print(f"File size: {len(content)} chars")

body = {
    "model": "qwen3.5-27b@q4_k_s",
    "input": [
        {
            "type": "message",
            "role": "user",
            "content": "I gave a tool the task to read a file. The result is below. Do you see LAST_LINE_MARKER in the tool result? Answer ONLY yes or no."
        },
        {
            "type": "function_call",
            "id": "fc_test1",
            "call_id": "call_test1",
            "name": "read_file",
            "arguments": json.dumps({"path": "test"})
        },
        {
            "type": "function_call_output",
            "call_id": "call_test1",
            "output": content
        }
    ],
    "tools": [{"type": "function", "name": "read_file", "description": "Read a file", "parameters": {"type": "object", "properties": {"path": {"type": "string"}}}}]
}

payload = json.dumps(body).encode("utf-8")
print(f"Request body size: {len(payload)} bytes")

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
                elif item.get("type") == "function_call":
                    print(f"Tool call requested: {item.get('name')}({item.get('arguments')})")
            print(f"Status: {data.get('status')}")
            usage = data.get("usage", {})
            print(f"Tokens: input={usage.get('input_tokens')}, output={usage.get('output_tokens')}")
        else:
            print(f"Full response: {json.dumps(data, indent=2)[:2000]}")
except urllib.error.HTTPError as e:
    body = e.read().decode("utf-8")
    print(f"HTTP {e.code}: {body[:2000]}")
