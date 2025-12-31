#!/usr/bin/env python3
import subprocess
import os
import re
import sys
import bisect

# --- Configuration ---
# 1 = Compact (no blank lines)
# 2 = Standard (1 blank line allowed)
MAX_CONSECUTIVE_NEWLINES = 1
TARGET_EXTENSIONS = {'.py', '.js', '.ts', '.c', '.cpp', '.rs', '.go', '.md', '.txt', '.lua'}

def get_changed_lines(filepath):
    """
    Returns a set of line numbers that have been added/modified 
    in the working tree relative to HEAD.
    """
    changed_lines = set()
    try:
        # -U0: Context size 0 (we only want the exact changed lines)
        # HEAD: Compare working tree against the last commit (covers staged and unstaged)
        cmd = ["git", "diff", "-U0", "HEAD", "--", filepath]
        result = subprocess.run(cmd, capture_output=True, text=True)
        
        # Regex to parse the hunk header: @@ -old_start,old_count +new_start,new_count @@
        # We only care about the '+' part which represents the current file state.
        hunk_re = re.compile(r'^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@')
        
        for line in result.stdout.splitlines():
            if line.startswith('@@'):
                match = hunk_re.match(line)
                if match:
                    start_line = int(match.group(1))
                    # If count is missing, it implies 1 line
                    count = int(match.group(2)) if match.group(2) else 1
                    
                    # Add all lines in this range to our set
                    # If count is 0, it's a deletion, so we skip (no lines in current file)
                    if count > 0:
                        for i in range(start_line, start_line + count):
                            changed_lines.add(i)
                            
    except Exception as e:
        print(f"Warning: Could not diff {filepath}: {e}")
    
    return changed_lines

def get_line_number(char_index, line_starts):
    """Binary search to find which line number a character index belongs to."""
    # bisect returns the insertion point. 
    # If index is 0, it returns 1 (line 1).
    return bisect.bisect_right(line_starts, char_index)

def clean_file(filepath):
    if not os.path.exists(filepath):
        return

    ext = os.path.splitext(filepath)[1]
    if ext not in TARGET_EXTENSIONS:
        return

    try:
        with open(filepath, 'r', encoding='utf-8') as f:
            content = f.read()
    except UnicodeDecodeError:
        return

    # 1. Get the specific lines that were changed in git
    changed_lines_set = get_changed_lines(filepath)
    if not changed_lines_set:
        return

    # 2. Map character offsets to line numbers
    # We store the start index of every line to quickly lookup line numbers later
    line_starts = [0] + [m.start() + 1 for m in re.finditer('\n', content)]

    original_content = content
    
    # 3. Find excessive newlines
    # Pattern: Look for more newlines than allowed.
    # If MAX=1, we look for \n\n+ (2 or more)
    pattern = r'\n{' + str(MAX_CONSECUTIVE_NEWLINES + 1) + ',}'
    
    # We iterate in reverse so replacements don't mess up indices of earlier matches
    matches = list(re.finditer(pattern, content))
    
    for match in reversed(matches):
        start_char = match.start()
        end_char = match.end()
        
        # Calculate which lines this whitespace block covers
        start_line = get_line_number(start_char, line_starts)
        end_line = get_line_number(end_char, line_starts)
        
        # Check for Intersection:
        # Does the range of lines occupied by this whitespace block overlap
        # with the lines we specifically changed in git?
        is_in_git_changes = False
        for i in range(start_line, end_line + 1):
            if i in changed_lines_set:
                is_in_git_changes = True
                break
        
        if is_in_git_changes:
            # Determine replacement
            replacement = '\n' * MAX_CONSECUTIVE_NEWLINES
            
            # Perform replacement via slicing
            content = content[:start_char] + replacement + content[end_char:]

    # 4. Write back if changed
    if content != original_content:
        with open(filepath, 'w', encoding='utf-8') as f:
            f.write(content)
        print(f"Cleaned changes in: {filepath}")

def main():
    # Get all files tracked by git that have changes (staged or unstaged)
    try:
        cmd = ["git", "diff", "--name-only", "HEAD"]
        result = subprocess.run(cmd, capture_output=True, text=True)
        files = result.stdout.splitlines()
    except FileNotFoundError:
        print("Git not found.")
        sys.exit(1)

    if not files:
        print("No changed files.")
        return

    for file in files:
        clean_file(file)

if __name__ == "__main__":
    main()