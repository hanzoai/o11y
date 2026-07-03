#!/bin/bash

echo "\n\n---\nRenamed tag files to index.ts...\n"
# Rename tag files to index.ts in services directories
# tags-split creates: services/tagName/tagName.ts -> rename to services/tagName/index.ts
find src/api/generated/services -mindepth 1 -maxdepth 1 -type d | while read -r dir; do
  dirname=$(basename "$dir")
  tagfile="$dir/$dirname.ts"
  if [ -f "$tagfile" ]; then
    mv "$tagfile" "$dir/index.ts"
    echo "Renamed $tagfile -> $dir/index.ts"
  fi
done

echo "\n✅ Tag files renamed to index.ts"

# Stabilize the shared schemas filename to `o11y.schemas.ts`.
# Orval derives it from the OpenAPI `info.title` ("Hanzo O11y" -> `hanzoO11y.schemas.ts`),
# but all consumers import `generated/services/o11y.schemas`. Keep the brand in the
# spec title and the import path stable by renaming the file + its internal imports.
echo "\n\n---\nStabilizing schemas filename to o11y.schemas.ts...\n"
if [ -f src/api/generated/services/hanzoO11y.schemas.ts ]; then
  mv src/api/generated/services/hanzoO11y.schemas.ts src/api/generated/services/o11y.schemas.ts
  grep -rl "hanzoO11y.schemas" src/api/generated | while read -r f; do
    perl -i -pe 's/hanzoO11y\.schemas/o11y.schemas/g' "$f"
  done
  echo "\n✅ Schemas file stabilized to o11y.schemas.ts"
fi

# Format generated files
echo "\n\n---\nRunning prettier...\n"
if ! pnpm prettify src/api/generated; then
  echo "Formatting failed!"
  exit 1
fi
echo "\n✅ Formatting successful"


# Fix linting issues
echo "\n\n---\nRunning lint...\n"
if ! pnpm lint:generated; then
  echo "Lint check failed! Please fix linting errors before proceeding."
  exit 1
fi
echo "\n✅ Lint check successful"


# Check for type errors
echo "\n\n---\nChecking for type errors...\n"
if ! tsc --noEmit; then
  echo "Type check failed! Please fix type errors before proceeding."
  exit 1
fi
echo "\n✅ Type check successful"


echo "\n\n---\n ✅✅✅ API generation complete!"
