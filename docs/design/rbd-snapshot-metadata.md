# CSI Snapshot-Metadata Sidecar for RBD deployment Design Document

[KEP-3314](https://github.com/kubernetes/enhancements/tree/master/keps/sig-storage/3314-csi-changed-block-tracking)
introduces new CSI APIs to identify changed blocks between snapshots of CSI volumes.
These APIs enable efficient and incremental backups by allowing backup applications
to retrieve only the data that has changed.

To support this feature, the Ceph-CSI should include a csi-snapshot-metadata
sidecar in the RBD controller deployment.

This document outlines the design for how the Ceph-CSI Operator manages the deployment
of the RBD controller with csi-snapshot-metadata sidecar and all required supporting resources.

(_Note: Only the RBD driver supports the SnapshotMetadata capability._)

## API Change

To enable the snapshot metadata sidecar in the RBD controller plugin,
a new field is added to the DriverSpec API:

```go
type DriverSpec struct {
  ...
  // Set to true to enable the snapshot metadata sidecar container in the RBD controller-plugin pod.
  // If the snapshotPolicy is set to none, this will be ignored.
  // +kubebuilder:validation:Optional
  // +kubebuilder:default:=false
  EnableSnapshotMetadata *bool `json:"enableSnapshotMetadata,omitempty"`
  ...
}
```

This field allows the users to explicitly opt in to deploying the
csi-snapshot-metadata sidecar in the RBD controller plugin.

> _Note: This field is only applicable for the RBD driver. For other driver types, the operator will ignore this flag._

## Expectations from the User

Before enabling the sidecar, the user must manually provision the following:

### 1. Provision of Certificates and TLS Secret

Before deploying the sidecar, the user must provision a valid server certificates
and private key for TLS communication. These must  be signed by trusted Certificate
Authority (CA). You may use:
- A certificate issued by a trusted organizational CA, or
- A self signed certificate, for guidance on generating self-signed certificates using
OpenSSL and preparing the TLS secret refer to this example [external-snapshot-metadata/deploy/example/csi-driver](https://github.com/kubernetes-csi/external-snapshot-metadata/tree/v0.1.0/deploy/example/csi-driver)

> _Note: Ensure that the certificate’s Common Name (CN) or Subject Alternative Name (SAN) matches the fully qualified DNS name of the Kubernetes Service you define in the next [section](#2-kubernetes-service). This DNS name must match the address field in the SnapshotMetadataService spec._

#### Requirements for the TLS Secret

- Must be named `<driverName>-metadata-tls` (_example: rbd.csi.ceph.com-metadata-tls_)
- Must be in the same namespace where the RBD Driver is deployed
- Must contain a valid certificate and private key, signed by a trusted CA.

Example:
```yaml
apiVersion: v1
kind: Secret
type: kubernetes.io/tls
metadata:
  name: rbd.csi.ceph.com-metadata-tls
  namespace: ceph-csi
data:
  tls.crt: <base64-encoded certificate>
  tls.key: <base64-encoded key>
```

### 2. Kubernetes Service
- Must expose port (`6443` or another desired port) and forward to the sidecar's internal port `50051`.
- Should have a selector matching the RBD controller pods `<driverName>-ctrlplugin` (_example: rbd.csi.ceph.com-ctrlplugin_)

Example:
```yaml
apiVersion: v1
kind: Service
metadata:
  name: rbd.csi.ceph.com-metadata
  namespace: ceph-csi
spec:
  ports:
    - name: snapshot-metadata-port
      port: 6443
      targetPort: 50051
  selector:
    app: rbd.csi.ceph.com-ctrlplugin
  type: ClusterIP
```

### 3. SnapshotMetadataService CR

To allow the sidecar to register and serve changed block information securely, the user must:

- Install the [SnapshotMetadataService CRD](https://raw.githubusercontent.com/kubernetes-csi/external-snapshot-metadata/refs/tags/v0.1.0/client/config/crd/cbt.storage.k8s.io_snapshotmetadataservices.yaml)
- Then, create the corresponding [SnapshotMetadataService CR](https://github.com/kubernetes/enhancements/blob/master/keps/sig-storage/3314-csi-changed-block-tracking/README.md#snapshot-metadata-service-custom-resource).

Define a SnapshotMetadataService resource corresponding to your RBD driver.
This resource must be named after the driver, e.g., rbd.csi.ceph.com, and should include:

- `address`: Fully qualified DNS name of the sidecar's Kubernetes service (e.g., rbd.csi.ceph.com-metadata.ceph-csi:6443)
- `audience`: A unique audience string used for verifying JWT-based client authentication. Ideally, this value should be unique to the service — for example, the DNS name of the sidecar’s service.
- `caCert`: Base64-encoded certificate authority that signed the sidecar’s server certificate

Example:
```yaml
apiVersion: cbt.storage.k8s.io/v1alpha1
kind: SnapshotMetadataService
metadata:
  name: rbd.csi.ceph.com
spec:
  address: rbd.csi.ceph.com-metadata.ceph-csi:6443
  audience: ceph-csi-snapshot-metadata
  caCert: <base64-encoded CA certificate>
```

> _Note: Only one SnapshotMetadataService CR should exist per driver._

## Operator Behavior

The operator includes the `csi-snapshot-metadata` sidecar only when all the following
conditions are met:

- The driver type is RBD
- The `enableSnapshotMetadata` flag is set to true in the Drivers API
- The snapshot policy is not `NoneSnapshotPolicy`

When the above conditions are satisfied, the operator adds the `csi-snapshot-metadata` sidecar container to the RBD controller pods

- Configures the sidecar with appropriate CLI flags:
  - --port=50051
  - --tls-cert=/certs/tls.crt
  - --tls-key=/certs/tls.key
- Mounts the `<driverName>-metadata-tls` (_example: `rbd.csi.ceph.com-metadata-tls`) secret at a predefined path (`/certs`).
