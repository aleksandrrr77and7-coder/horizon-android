import sys
data = open('.github/workflows/android-source.yml', 'rb').read()
print(f"Size: {len(data)} bytes")
print(f"First 10 bytes: {data[:10].hex()}")
print(f"First char: {chr(data[0])} (0x{data[0]:02x})")
print(f"All ASCII: {all(b < 128 for b in data)}")
