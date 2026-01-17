#!/usr/bin/env bash
set -euo pipefail

# remove-i18n-key.sh
# Usage:
#   scripts/remove-i18n-key.sh <key or parent.child> [glob]
# Example:
#   scripts/remove-i18n-key.sh fix_disconnect_tips
#   scripts/remove-i18n-key.sh battery_plugged.0
#   scripts/remove-i18n-key.sh sort_by.name_desc 'web/src/locales/*.ts'

if [[ $# -lt 1 ]]; then
  echo "Usage: $0 <key or parent.child> [glob]" 1>&2
  exit 1
fi

KEY="$1"
GLOB="${2:-web/src/locales/*.ts}"

PARENT="$KEY"
CHILD=""
if [[ "$KEY" == *.* ]]; then
  PARENT="${KEY%%.*}"
  CHILD="${KEY#*.}"
fi

for f in $GLOB; do
  [[ -f "$f" ]] || continue
  tmp="$f.tmp.$$"
  awk -v parentKey="$PARENT" -v childKey="$CHILD" '
    function startsWithKey(line, key,   re1,re2) {
      re1 = "^[[:space:]]*" key "[[:space:]]*:[[:space:]]*"      # unquoted key
      re2 = "^[[:space:]]*[\x27\"];?" key "[\x27\"];?[[:space:]]*:[[:space:]]*" # quoted key (single/double)
      return (line ~ re1) || (line ~ re2)
    }

    BEGIN {
      skippingBlock = 0
      blockBalance = 0
      inParent = 0
      parentBalance = 0
    }

    {
      line = $0

      # compute opens/closes on a copy to avoid altering $0
      tmp = line
      opens = gsub(/\{/, "{", tmp)
      closes = gsub(/\}/, "}", tmp)

      if (skippingBlock) {
        blockBalance += opens - closes
        if (blockBalance <= 0) {
          skippingBlock = 0
          blockBalance = 0
        }
        next
      }

      if (childKey == "") {
        # delete top-level property
        if (startsWithKey(line, parentKey)) {
          if (line ~ /:[[:space:]]*\{[[:space:]]*$/) {
            # value is an object, skip until closing brace line (handles nested)
            skippingBlock = 1
            blockBalance = 1
            next
          }
          # simple value on one line
          next
        }
        print line
        next
      }

      # nested deletion: enter parent object
      if (!inParent && startsWithKey(line, parentKey) && line ~ /:[[:space:]]*\{[[:space:]]*$/) {
        inParent = 1
        parentBalance = 1
        print line
        next
      }

      if (inParent) {
        # within parent block
        if (parentBalance == 1 && startsWithKey(line, childKey)) {
          # remove direct child property (assumed one-line value)
          # if child value starts a nested object, remove whole nested object
          if (line ~ /:[[:space:]]*\{[[:space:]]*$/) {
            skippingBlock = 1
            blockBalance = 1
            next
          }
          next
        }
        print line
        parentBalance += opens - closes
        if (parentBalance <= 0) {
          inParent = 0
          parentBalance = 0
        }
        next
      }

      print line
    }
  ' "$f" > "$tmp"
  mv "$tmp" "$f"
done

echo "Done."


