#!/usr/bin/env bash
# DO NOT run this script on its own
# It will append the required template multiple times
# It is intended to be ran as a part of generating heml charts using helmify

set -e

CHART_DIR=$1
SA_FILE="${CHART_DIR}/templates/serviceaccount.yaml"

if [ ! -f "$SA_FILE" ]; then
  echo "No serviceaccount.yaml template in $CHART_DIR, nothing to do!"
  exit 1
fi

TEMPLATE_SNIPPET=$(
  cat <<'EOF'
{{- with .Values.imagePullSecrets }}
imagePullSecrets:
{{ toYaml . | indent 2 }}
{{- end }}
EOF
)

echo "Patching $SA_FILE for imagePullSecrets"

awk -v snippet="$TEMPLATE_SNIPPET" '
  # Before printing a separator, print the snippet
  /^---/ { print snippet }

  # Print the current line
  { print }

  # At the end of the file, print the snippet for the last section
  END { print snippet }
' "$SA_FILE" >"${SA_FILE}.tmp"

mv "${SA_FILE}.tmp" "$SA_FILE"

echo "Patched $SA_FILE successfully"
exit 0
