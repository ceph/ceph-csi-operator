# Ceph CSI Snapshot-Metadata Sidecar deployment Design Document

[KEP-3314](https://github.com/kubernetes/enhancements/tree/master/keps/sig-storage/3314-csi-changed-block-tracking)
introduces new CSI APIs to identify changed blocks between CSI volume snapshots.
These APIs enable efficient and incremental backups by allowing backup applications
to retrieve only the data that has changed.

To support this feature, the Ceph-CSI project should include a csi-snapshot-metadata
sidecar in the RBD controller deployment. This sidecar serves a gRPC server over TLS,
providing metadata about changed blocks in snapshots.


This document outlines the design for how the Ceph-CSI Operator manages the deployment
of the RBD controller with csi-snapshot-metadata sidecar and all required supporting resources.

(_Note: Only the RBD driver supports the SnapshotMetadata capability._)

## Resources created by the ceph-csi-operator

1. [TLS Certificates and Kubernetes Secret](#provision-of-certificates-and-kubernetes-tls-secret)
2. [Kubernetes Service for the Sidecar](#kubernetes-service)
3. [SnapshotMetadataService Custom Resource](#snapshotmetadataservice-custom-resource)
4. [RBD Controller Deployment with Sidecar](#deployment-of-rbd-driver-deployment-with-csi-snapshot-metadata-sidecar)


## Provision of Certificates and Kubernetes TLS Secret

- The operator checks for the existence of a TLS secret named `csi-snapshot-metadata-server-certs`.
- If not present:
  - A **self-signed CA**, server certificate, and private key are generated.
  - A Kubernetes TLS secret is created containing the certificate and key.
- If present:
  - The secret's certificate is evaluated for expiration.
  - If the expiry is within 24 hours, the certificate is regenerated and the secret is updated.
- The expiry date is stored in the reconciler struct for requeue logic.

(_Note: Certificates are created with an expiry of 1 year after which these needs to be renewed for continued operations._)

```go
type driverReconcile struct {
	DriverReconciler
	ctx                        context.Context
	log                        logr.Logger
	driver                     csiv1a1.Driver
	driverType                 DriverType
	images                     map[string]string
	snapshotMetadataCertExpiry time.Time            // store the certificate expiry date
}
```

To ensure certificate renewal happens before expiration, the reconciler schedules itself to re-run 24 hours prior:
```go
return ctrl.Result{
    Requeue: true,
    RequeueAfter: snapshotMetadataCertExpiry.Add(-24 * time.Hour), // Requeue 24 hours before expiry
}
```

(_Note: The csi-snapshot-metadata sidecar watches for TLS secret changes.On certificate update, it will automatically restart the gRPC server with new credentials. No pod restart is necessary._)

Example TLS secret
```yaml
apiVersion: v1
kind: Secret
type: kubernetes.io/tls
metadata:
  name: csi-snapshot-metadata-server-certs
data:
  tls.crt: <base64-encoded server certificate>
  tls.key: <base64-encoded private key>
```

## Kubernetes Service

A ClusterIP service is required to expose the snapshot metadata gRPC server to internal clients (e.g., backup applications).

Example K8s service -
```yaml
apiVersion: v1
kind: Service
metadata:
  name: ceph-csi-operator-snapshot-metadata
  namespace: ceph-csi-operator-system
spec:
  ports:
    - name: snapshot-metadata-port
      port: 6443
      protocol: TCP
      targetPort: 50051  # Matches the csi-snapshot-metadata sidecar's --port flag
  selector:
    app: rbd.csi.ceph.com-ctrlplugin
  type: ClusterIP
```


## SnapshotMetadataService Custom Resource

To advertise the presence of the snapshot metadata gRPC service,
the CSI driver must create a SnapshotMetadataService CR. This CR includes:

- The gRPC server endpoint
- The CA certificate
- The token audience string for authentication

_Naming convention: The CR name should match the CSI driver's provisioner name (rbd.csi.ceph.com)._

Example Custom Resource
```yaml
apiVersion: cbt.storage.k8s.io/v1alpha1
kind: SnapshotMetadataService
metadata:
  name: rbd.csi.ceph.com
spec:
  address: ceph-csi-operator-snapshot-metadata.ceph-csi-operator-system:6443 # <svc-name>.<svc-namespace>:<svc-port>
  audience: ceph-csi-operator-snapshot-metadata
  caCert: <base64-encoded CA certificate>
```

## Deployment of RBD driver deployment with csi-snapshot-metadata sidecar

The final step involves deploying the RBD controller along with the
`csi-snapshot-metadata` sidecar. This sidecar must:

- Be configured with the necessary flags (e.g., `--port`, `--tls-cert`, `--tls-key`).
- Mount the provisioned TLS secret.

The Ceph-CSI Operator conditionally includes this sidecar in the deployment only when
all of the following criteria are met:

- The deployment is for RBD driver(`r.isRbdDriver()`)
- The `SnapshotMetadataService` CRD is installed and accessible
- The `enableSnapshotMetadata` flag is set to `true`
- The snapshot policy is **not** `NoneSnapshotPolicy`

This conditional logic ensures the sidecar is added only in supported
and properly configured environments, reducing unnecessary resource usage and complexity.
