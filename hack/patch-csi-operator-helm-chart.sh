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

# Replace hardcoded namespace with Helm template variable in all template files
if [[ "$CHART_DIR" == *"ceph-csi-operator"* ]]; then
  echo "Replacing hardcoded namespace with Helm template variable in $CHART_DIR"

  # Process each YAML file in the templates directory
  find "${CHART_DIR}/templates" -type f -name "*.yaml" | while read -r file; do
    # Check if the file contains the namespace string to replace
    if grep -q "namespace: ceph-csi-operator-system" "$file"; then
      echo "Processing $file"

      # Check if the file already has the $root variable definition
      if ! grep -q "{{- \$root := . -}}" "$file"; then
        # Add the $root variable definition at the top of the file
        sed -i "1i{{- \$root := . -}}" "$file"
      fi

      # Replace the namespace string
      sed -i "s/namespace: ceph-csi-operator-system/namespace: {{ \$root.Release.Namespace }}/g" "$file"
    fi
  done

  echo "Namespace replacement completed successfully"
fi

exit 0
