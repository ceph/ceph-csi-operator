# Guide for RBD Deployment with CSI Snapshot-Metadata Sidecar

> ⚠️ **Warning - Alpha Feature**:
> This feature is currently in **alpha** and is subject to change in future releases.
> This feature should only be used for testing and evaluation purposes.

[KEP-3314](https://github.com/kubernetes/enhancements/tree/master/keps/sig-storage/3314-csi-changed-block-tracking)
introduces new CSI APIs to identify changed blocks between snapshots of CSI volumes.
These APIs enable efficient and incremental backups by allowing backup applications
to retrieve only the data that has changed.

To support this feature, Ceph-CSI should include an `external-snapshot-metadata`
sidecar in the RBD controller plugin.

This document outlines how the Ceph-CSI Operator manages the deployment
of the RBD controller plugin with the `external-snapshot-metadata` sidecar.

(_Note: Only the RBD driver supports the SnapshotMetadata capability._)

## Admin Responsibilities

Users need to perform the following manual setup:

1. Install the [SnapshotMetadataService CRD](https://github.com/kubernetes-csi/external-snapshot-metadata/blob/v0.1.0/client/config/crd/cbt.storage.k8s.io_snapshotmetadataservices.yaml)

   ```bash
   kubectl create -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshot-metadata/refs/tags/v0.1.0/client/config/crd/cbt.storage.k8s.io_snapshotmetadataservices.yaml
   ```

2. Create a Service to expose the RBD driver pod

   Create a service to enable communication with the RBD controller plugin:

   ```yaml
   apiVersion: v1
   kind: Service
   metadata:
     name: <service-name>
     namespace: <driver-namespace>
   spec:
     ports:
     - name: snapshot-metadata-port
       port: <service-port>
       protocol: TCP
       targetPort: 50051  # should be the same as the sidecar uses this port for its gRPC server
     selector:
       app: <driver-name>-ctrlplugin  # RBD controller plugin pod label
     type: ClusterIP
   ```

   > **Note:**
   > - Replace `<service-name>` with your desired service name (e.g., `rbd-csi-ceph-com-metadata`)
   > - Replace `<driver-namespace>` with the namespace where your RBD driver is deployed
   > - Replace `<service-port>` with your desired service port (e.g., `6443`)
   > - Replace `<driver-name>` with your RBD driver name (e.g., `rbd.csi.ceph.com`)

3. Provision TLS certificates and create a TLS secret

   Generate TLS certificates using your preferred method (self-signed, cert-manager, etc).
   The certificates must be valid for the service domain created in step 2: `<service-name>.<driver-namespace>`

   Create a TLS secret with the generated certificates:

   ```bash
   kubectl create secret tls <driver-name> \
     --namespace=<driver-namespace> \
     --cert=server-cert.pem \
     --key=server-key.pem
   ```

   > **Note:**
   > - Replace `<driver-name>` with your RBD driver name (e.g., `rbd.csi.ceph.com`)
   > - Replace `<driver-namespace>` with the namespace where your RBD driver is deployed
   > - Ensure certificates are valid for the domain: `<service-name>.<driver-namespace>` (using the service name from step 2)

4. Create a SnapshotMetadataService CR for the RBD driver that will deploy the `external-snapshot-metadata` sidecar.
   The name of this CR must match the RBD driver CR name.

   **Example:**

   ```yaml
   apiVersion: cbt.storage.k8s.io/v1alpha1
   kind: SnapshotMetadataService
   metadata:
     name: <driver-name>
   spec:
     address: <service-name>.<driver-namespace>:<service-port>
     audience: <driver-name>
     caCert: <ca-bundle>
   ```

   > **Note:**
   > - `address`: Should point to the service created in step 2, replace `<service-name>`, `<driver-namespace>`, and `<service-port>` with your actual values from step 2
   > - `audience`: Recommended to use the CSI driver name for consistency
   > - `caCert`: Base64-encoded CA certificate bundle

5. Provide the TLS secret required for the `external-snapshot-metadata` sidecar as a volume mount in the RBD driver CR.

   **Example:**

   ```yaml
   apiVersion: csi.ceph.io/v1
   kind: Driver
   metadata:
     name: <driver-name>
     namespace: <driver-namespace>
   spec:
     # ... other fields ...
     controllerPlugin:
       volumes:
       - mount:
           mountPath: /tmp/certificates  # Must be /tmp/certificates - required by sidecar
           name: tls-key 
         volume:
           name: tls-key # Must be "tls-key"
           secret:
             secretName: snapshot-metadata-tls  # The TLS secret name
   ```

   > **Note:**
   > - **mountPath must be `/tmp/certificates`**: This path is required by the snapshot metadata sidecar to locate TLS certificates.
   > - **Volume name and mount name must be `tls-key`**: The operator specifically filters for volumes with this exact name to mount in the snapshot-metadata sidecar container.
   > - Replace `<driver-name>` with your RBD driver name (e.g., `rbd.csi.ceph.com`)
   > - Replace `<driver-namespace>` with the namespace where your RBD driver is deployed

## Ceph-CSI Operator Responsibilities

The operator will perform the following actions for the RBD controller plugin deployment:

- Check for the existence of the SnapshotMetadataService CR (name must be the same as the RBD driver CR)
- Check for the volume of type SecretVolumeSource (name must be the same as the RBD driver CR)

> ⚠️ **Note**: If the SnapshotMetadataService CR is created after adding the volume configuration
> in the RBD driver CR, the ceph-csi-operator pod needs to be restarted manually.
