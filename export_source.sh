#!/bin/bash

# ==============================================================================
# Script to export project source files into a single text file.
#
# This script is designed for AI analysis tools like NotebookLM. It finds all
# specified file types, adds a separator header for each file, and then
# concatenates them into a single output file.
# ==============================================================================

# --- Configuration ---
# Set the name of the final output file.
OUTPUT_FILE="go_project_source_$(date +'%Y%m%d_%H%M%S').txt"

# --- Start of Script ---
echo "🚀 Starting to export source code..."
echo "Output will be saved to: $OUTPUT_FILE"

# The main logic wrapped in a block to redirect all output to the file at once.
# This is more efficient than appending for each file.
{
  # Use 'find' to locate all relevant files.
  # -path ./client -prune: Tìm thư mục 'client' và "prune" (cắt tỉa) nó,
  #                        nghĩa là 'find' sẽ không đi vào bên trong thư mục này.
  # -o: OR (hoặc), tiếp tục với logic tìm kiếm còn lại cho các đường dẫn khác.
  # -type f: only find files.
  # \( ... \): group conditions.
  # -name "*.go": find files ending in .go.
  # -o: OR condition.
  # -print0: prints the full file path followed by a null character,
  #          which handles filenames with spaces or special characters safely.
  find . -path ./client -prune -o -type f \( -name "*.go" -o -name "*.md" -o -name "go.mod" -o -name "go.sum" \) -print0 | \

  # Pipe the null-separated list of files to a 'while' loop.
  # IFS= read -r -d '': reads the input safely, respecting special characters.
  while IFS= read -r -d '' file; do
    # Print a separator header for clarity.
    echo "--- FILE: ${file} ---"

    # Append the content of the actual file.
    cat "${file}"

    # Add a newline for better spacing between file contents.
    echo
  done

} > "$OUTPUT_FILE" # Redirect the entire output of the block to the output file.

echo "✅ Done! All source code has been successfully exported."