import sys
import os
import re

# Suppress warnings
os.environ['TF_CPP_MIN_LOG_LEVEL'] = '3' 

try:
    import ddddocr
except ImportError:
    print("Error: ddddocr module not found.", file=sys.stderr)
    sys.exit(1)

def is_valid(s):
    return len(s) == 4 and s.isalnum()

def solve():
    try:
        # Read raw bytes from stdin buffer
        img_bytes = sys.stdin.buffer.read()
        
        if not img_bytes:
            print("Error: No image data received.", file=sys.stderr)
            sys.exit(1)

        ocr = ddddocr.DdddOcr(show_ad=False)
        res = ocr.classification(img_bytes)
        
        if is_valid(res):
            sys.stdout.write(res)
            return

        # Retry with beta
        ocr_beta = ddddocr.DdddOcr(show_ad=False, beta=True)
        res_beta = ocr_beta.classification(img_bytes)
        
        if is_valid(res_beta):
            sys.stdout.write(res_beta)
            return
            
        # Fallback to original result if neither is "perfect", 
        # as it might just be a hard captcha but correct OCR.
        # Or maybe beta is better on average for hard ones?
        # Let's return the one that looks "more" valid? 
        # Hard to say. Return original.
        sys.stdout.write(res)

    except Exception as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(1)

if __name__ == "__main__":
    solve()
