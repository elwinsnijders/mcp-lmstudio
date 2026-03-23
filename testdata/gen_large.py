lines = []
lines.append("// FIRST_LINE_MARKER - file size should be approximately 100KB")
lines.append("")
lines.append("package testdata")
lines.append("")

pad = " " * 16

for i in range(1, 198):
    num = f"{i:03d}"
    lines.append(f"func processItem_{num}(data []byte) ([]byte, error) {{ // processItem_{num} xor transform{pad}")
    lines.append(f"    result := make([]byte, len(data)) // allocate result buffer same length as input data{pad}")
    lines.append(f"    for i := range data {{ result[i] = data[i] ^ 0x42 }} // xor each byte with constant 0x42{pad}")
    lines.append(f"    return result, nil // return transformed slice and nil error on success path here{pad}")
    lines.append(f"}} // end processItem_{num}{pad * 3}")
    if i == 80:
        lines.append("// === MARKER_AT_40KB === (byte offset ~40000)")
    if i == 100:
        lines.append("// === MARKER_AT_50KB === (byte offset ~50000)")
    if i == 120:
        lines.append("// === MARKER_AT_60KB === (byte offset ~60000)")
    if i == 160:
        lines.append("// === MARKER_AT_80KB === (byte offset ~80000)")

lines.append("// === MARKER_AT_100KB === (byte offset ~100000)")
lines.append("// LAST_LINE_MARKER - if you see this, the file was read completely")

content = "\n".join(lines)
print(f"Generated size: {len(content)} bytes")
print(f"Generated lines: {len(lines)}")

with open(r"C:\Users\Elwin\projects\mcp-lmstudio\testdata\large_test.go", "w", newline="\n") as f:
    f.write(content)
print("Written successfully")
