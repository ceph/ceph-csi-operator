# v1.2.0 Pending Release Notes

## Breaking Changes

## Features

- Support pod-level resource requests and limits for driver pods through the
  `podResources` field of the `nodePlugin` and `controllerPlugin` specs, and
  the corresponding `ceph-csi-drivers` Helm chart values.

## NOTE

The default `csi-snapshot-metadata` sidecar image is updated to `v1.2.0`.
