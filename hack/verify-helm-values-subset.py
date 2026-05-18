#!/usr/bin/env python3
"""
Verify that all generated helm values exist in the maintained values.yaml file.
Allows the maintained file to have additional fields (like openshift.enabled)
while ensuring all generated fields are present and have matching values.
"""

import sys
import yaml

# Fields that are allowed to have different values between generated and maintained
# These are fields where we use a placeholder in the kustomize patch for helmify
# but want a more meaningful default in the maintained values.yaml
ALLOWED_VALUE_DIFFS = {
    "controllerManager.priorityClassName",
    "controllerManager.manager.image.tag",
}


def check_subset(generated, maintained, path=""):
    """
    Recursively check that generated is a subset of maintained.

    Args:
        generated: The generated values (from helmify)
        maintained: The manually maintained values
        path: Current path in the YAML structure (for error reporting)

    Returns:
        True if generated is a subset of maintained, False otherwise
    """
    if isinstance(generated, dict) and isinstance(maintained, dict):
        for key, value in generated.items():
            full_path = f"{path}.{key}" if path else key

            if key not in maintained:
                print(f"ERROR: Missing key in maintained file: {full_path}")
                return False

            if not check_subset(value, maintained[key], full_path):
                return False

    elif isinstance(generated, list) and isinstance(maintained, list):
        if len(generated) != len(maintained):
            print(f"ERROR: List length mismatch at {path}: generated has {len(generated)} items, maintained has {len(maintained)} items")
            return False

        for i, (gen_item, maint_item) in enumerate(zip(generated, maintained)):
            if not check_subset(gen_item, maint_item, f"{path}[{i}]"):
                return False

    elif generated != maintained:
        # Check if this field is allowed to have different values
        if path in ALLOWED_VALUE_DIFFS:
            # Field is allowed to differ - just verify the key exists
            return True
        print(f"ERROR: Value mismatch at {path}")
        print(f"  Generated: {generated}")
        print(f"  Maintained: {maintained}")
        return False

    return True


def main():
    if len(sys.argv) != 3:
        print("Usage: verify-helm-values-subset.py <generated-file> <maintained-file>")
        sys.exit(1)

    generated_file = sys.argv[1]
    maintained_file = sys.argv[2]

    try:
        with open(generated_file, 'r') as f:
            generated = yaml.safe_load(f)

        with open(maintained_file, 'r') as f:
            maintained = yaml.safe_load(f)

        if not check_subset(generated, maintained):
            print("\nThe maintained values.yaml file is missing required fields or has mismatched values.")
            print("Note: The maintained file CAN have additional fields (e.g., openshift.enabled),")
            print("but all generated fields must be present and match.")
            sys.exit(1)

        print("✓ All generated values are present in the maintained file.")
        sys.exit(0)

    except Exception as e:
        print(f"ERROR: Failed to verify helm values: {e}")
        sys.exit(1)


if __name__ == "__main__":
    main()
