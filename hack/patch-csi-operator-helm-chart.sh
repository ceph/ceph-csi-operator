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

echo "Patching $SA_FILE for imagePullSecrets"

# Use awk to insert the snippet before each '---' separator and at the end
awk '
  # Before printing a separator, print the snippet
  /^---/ {
    print "{{- with .Values.imagePullSecrets }}"
    print "imagePullSecrets:"
    print "{{ toYaml . | indent 2 }}"
    print "{{- end }}"
  }

  # Print the current line
  { print }

  # At the end of the file, print the snippet for the last section
  END {
    print "{{- with .Values.imagePullSecrets }}"
    print "imagePullSecrets:"
    print "{{ toYaml . | indent 2 }}"
    print "{{- end }}"
  }
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
        # Add the $root variable definition at the top of the file using a temp file
        echo "{{- \$root := . -}}" | cat - "$file" > "$file.tmp" && mv "$file.tmp" "$file"
      fi

      # Replace the namespace string using a temp file (portable across all platforms)
      sed "s/namespace: ceph-csi-operator-system/namespace: {{ \$root.Release.Namespace }}/g" "$file" > "$file.tmp" && mv "$file.tmp" "$file"
    fi
  done

  echo "Namespace replacement completed successfully"
fi

exit 0
