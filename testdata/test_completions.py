import json
import urllib.request

with open(r"C:\Users\Elwin\projects\mcp-lmstudio\testdata\large_test.go", "r") as f:
    content = f.read()

print(f"File size: {len(content)} chars")

body = {
    "model": "qwen3.5-27b@q4_k_s",
    "messages": [
        {"role": "user", "content": "I gave a tool the task to read a file. The result is below. Do you see LAST_LINE_MARKER in the tool result? Answer ONLY yes or no."},
        {"role": "assistant", "content": None, "tool_calls": [{"id": "test1", "type": "function", "function": {"name": "read_file", "arguments": json.dumps({"path": "test"})}}]},
        {"role": "tool", "content": content, "tool_call_id": "test1"}
    ],
    "tools": [{"type": "function", "function": {"name": "read_file", "description": "Read a file", "parameters": {"type": "object", "properties": {"path": {"type": "string"}}}}}]
}

payload = json.dumps(body).encode("utf-8")
print(f"Request body size: {len(payload)} bytes")

req = urllib.request.Request(
    "http://localhost:1234/v1/chat/completions",
    data=payload,
    headers={"Content-Type": "application/json", "Authorization": "Bearer sk-lm-K52VcdOs:8ewvJcZ0vK9gaEILPghx"},
    method="POST"
)
try:
    with urllib.request.urlopen(req, timeout=300) as resp:
        data = json.loads(resp.read().decode("utf-8"))
        if "choices" in data:
            msg = data["choices"][0]["message"]
            answer = msg.get("content", "(no content)")
            print(f"Model answer: {answer}")
            print(f"Finish reason: {data['choices'][0]['finish_reason']}")
            usage = data.get("usage", {})
            print(f"Tokens: input={usage.get('prompt_tokens')}, output={usage.get('completion_tokens')}")
        else:
            print(f"Error: {json.dumps(data, indent=2)}")
except urllib.error.HTTPError as e:
    print(f"HTTP {e.code}: {e.read().decode('utf-8')}")
