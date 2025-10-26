#!/bin/bash

# ==============================================================================
# Script to export project source code into a single text file.
#
# This script is designed for AI analysis tools (like NotebookLM).
# It finds all specified file types (React, TS, JS, Config, etc.),
# adds a separator header for each file, and then concatenates them
# into a single output file.
# ==============================================================================

# --- Configuration ---
# Set the name of the final output file.
OUTPUT_FILE="react_project_source_$(date +'%Y%m%d_%H%M%S').txt"

# Directories to ignore (node_modules is the most important).
# Add other build or cache directories if needed.
EXCLUDE_DIRS=(
  "./node_modules"
  "./dist"
  "./build"
  "./.git"
  "./.next"
  "./.cache"
)

# --- CHANGED: Split files into groups for sorting ---
# UI files (will be exported first)
UI_FILES=(
  -name "*.tsx"
  -o -name "*.jsx"
  -o -name "*.html"
  -o -name "*.css"
  -o -name "*.scss"
  -o -name "*.module.css"
  -o -name "*.module.scss"
)

# Other files (config, logic, docs, etc.)
OTHER_FILES=(
  -name "*.ts"
  -o -name "*.js"
  -o -name "*.md"
  -o -name "package.json"
  -o -name "package-lock.json"
  -o -name "pnpm-lock.yaml"
  -o -name "tsconfig.json"
  -o -name "vite.config.*"
  -o -name "webpack.config.*"
  -o -name "next.config.*"
  -o -name ".eslintrc.*"
  -o -name "prettier.config.*"
  -o -name "tailwind.config.*"
  -o -name "*.json"
)

# --- Start of Script ---
echo "🚀 Starting to export source code (recursively)..."
echo "Sorting UI files first."
echo "Output file will be saved to: $OUTPUT_FILE"

# --- Build the 'find' command ---
# We build arguments in an array to avoid 'eval' and handle spaces safely.
BASE_FIND_ARGS=(".")

# 2. Build exclusion logic (EXCLUDE_DIRS)
EXCLUDE_ARGS=()
for dir in "${EXCLUDE_DIRS[@]}"; do
  if [ ${#EXCLUDE_ARGS[@]} -eq 0 ]; then
    # First item, no '-o'
    EXCLUDE_ARGS+=(-path "$dir")
  else
    # Subsequent items, add '-o'
    EXCLUDE_ARGS+=(-o -path "$dir")
  fi
done

# Add the exclusion logic to the main args array
if [ ${#EXCLUDE_ARGS[@]} -gt 0 ]; then
  # Note: \( ... \) are added as separate arguments for find
  BASE_FIND_ARGS+=(\( "${EXCLUDE_ARGS[@]}" \) -prune -o)
fi

# --- NEW: Helper function to find and append files ---
# $1: Title for the section (used for echo)
# $2...: Array of include patterns
append_files_to_output() {
  local title="$1"
  shift # Remove title from arguments
  local include_patterns=("$@")

  # Build the final find command for this group
  local CURRENT_FIND_ARGS=("${BASE_FIND_ARGS[@]}")
  CURRENT_FIND_ARGS+=(-type f \( "${include_patterns[@]}" \) -print0)

  echo "--- Finding $title files... ---"

  # Run find and pipe to the loop
  find "${CURRENT_FIND_ARGS[@]}" | \
  {
    while IFS= read -r -d '' file; do
      # Print a separator header for the file
      echo "--- FILE: ${file} ---"
      # Concatenate the file's content
      cat "${file}"
      # Add a newline for clear separation
      echo
    done
  } # This block's output is redirected
}

# --- Run the export logic ---
# 1. Append UI files first (redirect > to create/overwrite the file)
{
  append_files_to_output "UI" "${UI_FILES[@]}"
} > "$OUTPUT_FILE"

# 2. Append OTHER files (redirect >> to append to the file)
{
  append_files_to_output "Other" "${OTHER_FILES[@]}"
} >> "$OUTPUT_FILE"


echo "✅ Done! All source code has been successfully exported."
