import sys
import os

# Suppress warnings
os.environ['TF_CPP_MIN_LOG_LEVEL'] = '3' 

try:
    import ddddocr
except ImportError:
    print("Error: ddddocr module not found.", file=sys.stderr)
    sys.exit(1)

def solve():
    try:
        # Read raw bytes from stdin buffer
        img_bytes = sys.stdin.buffer.read()
        
        if not img_bytes:
            print("Error: No image data received.", file=sys.stderr)
            sys.exit(1)

        ocr = ddddocr.DdddOcr(show_ad=False)
        res = ocr.classification(img_bytes)
        
        # Print result to stdout (no newline to avoid extra chars, or strip in Go)
        sys.stdout.write(res)
    except Exception as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(1)

if __name__ == "__main__":
    solve()
