# v1.1.0 Pending Release Notes

## Breaking Changes

## Features

- Added NetworkPolicies for the operator pod and CSI driver pods (controller-plugin, csi-addons nodeplugin). Included in all generated manifests by default. Driver pod NPs are created by the operator for every reconciled driver. Node-plugin pods are exempt (`hostNetwork: true`).
- The CSI driver liveness sidecar can now be configured when deploying with Helm

## NOTE
