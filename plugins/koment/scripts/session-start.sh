#!/bin/sh
set -eu

if policy_status=$(koment agents check 2>&1); then
  printf '%s\n' "$policy_status"
else
  printf 'koment policy needs attention before editing:\n%s\n' "$policy_status"
fi

printf '%s\n' \
  'STRICT KOMENT PROCEDURE:' \
  '1. Call koment_get for every existing file before editing it.' \
  '2. Call koment_search before changing a non-obvious decision.' \
  '3. Treat drifted, orphaned, and ambiguous annotations as history, not current truth.' \
  '4. Do not add explanatory source comments. Rename, extract, introduce a named type or constant, restructure, then call koment_add.' \
  '5. Convert completed comment intent with koment_convert_comment. Keep a comment only with koment_acknowledge_comment and explicit acknowledgement.' \
  '6. Do not finish until koment check, koment comments check, and koment agents check all pass.'
