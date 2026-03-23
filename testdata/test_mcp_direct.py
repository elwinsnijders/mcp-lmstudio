import subprocess
import json
import sys

proc = subprocess.Popen(
    [
        r"C:\Users\Elwin\projects\mcp-console\mcp-console.exe",
        "--workspace", r"C:\Users\Elwin\projects",
        "--config", r"C:\Users\Elwin\projects\mcp-console\config.yaml",
    ],
    stdin=subprocess.PIPE,
    stdout=subprocess.PIPE,
    stderr=subprocess.PIPE,
    env={
        "PATH": r"C:\Program Files\Git\usr\bin;C:\Program Files\Git\cmd;C:\Windows\system32;C:\Windows",
        "SystemRoot": r"C:\Windows",
    },
)

def send(msg):
    line = json.dumps(msg) + "\n"
    proc.stdin.write(line.encode())
    proc.stdin.flush()

def recv():
    line = proc.stdout.readline()
    if not line:
        err = proc.stderr.read().decode()
        print(f"STDERR: {err}", file=sys.stderr)
        return None
    return json.loads(line)

send({
    "jsonrpc": "2.0",
    "id": 1,
    "method": "initialize",
    "params": {
        "protocolVersion": "2024-11-05",
        "capabilities": {},
        "clientInfo": {"name": "test", "version": "1.0"},
    },
})
resp = recv()
print(f"Init response: {json.dumps(resp)[:200]}")

send({"jsonrpc": "2.0", "method": "notifications/initialized"})

send({
    "jsonrpc": "2.0",
    "id": 2,
    "method": "tools/call",
    "params": {
        "name": "read_file",
        "arguments": {
            "path": r"C:\Users\Elwin\projects\mcp-lmstudio\testdata\large_test.go",
        },
    },
})
resp = recv()

if resp and "result" in resp:
    content_parts = resp["result"].get("content", [])
    total = 0
    has_last_marker = False
    has_first_marker = False
    has_truncated = False
    for part in content_parts:
        text = part.get("text", "")
        total += len(text)
        if "LAST_LINE_MARKER" in text:
            has_last_marker = True
        if "FIRST_LINE_MARKER" in text:
            has_first_marker = True
        if "truncated" in text.lower():
            has_truncated = True
    print(f"Content size from mcp-console: {total} chars")
    print(f"Contains FIRST_LINE_MARKER: {has_first_marker}")
    print(f"Contains LAST_LINE_MARKER: {has_last_marker}")
    print(f"Contains 'truncated': {has_truncated}")
    last_200 = ""
    for part in content_parts:
        last_200 = part.get("text", "")[-200:]
    print(f"Last 200 chars: {repr(last_200)}")
elif resp:
    print(f"Error response: {json.dumps(resp, indent=2)}")
else:
    print("No response received")

proc.terminate()
