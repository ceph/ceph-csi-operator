# Customizing Container Images

The Ceph-CSI Operator allows you to customize the container images used by the CSI drivers through a ConfigMap-based image set configuration. This is particularly useful for:

- **Disconnected/Air-gapped deployments**: Deploy from a private registry without internet access
- **Custom image builds**: Use your own builds of the CSI components
- **Version pinning**: Lock specific component versions for stability
- **Registry mirroring**: Use a local mirror of public registries

## Available Image Keys

The operator uses the following image keys in the ConfigMap. Each key corresponds to a specific container in the CSI driver deployment:

| Key | Description | Default Image |
|-----|-------------|---------------|
| `provisioner` | CSI external-provisioner sidecar | `registry.k8s.io/sig-storage/csi-provisioner:v6.2.0` |
| `attacher` | CSI external-attacher sidecar | `registry.k8s.io/sig-storage/csi-attacher:v4.12.0` |
| `resizer` | CSI external-resizer sidecar | `registry.k8s.io/sig-storage/csi-resizer:v2.1.0` |
| `snapshotter` | CSI external-snapshotter sidecar | `registry.k8s.io/sig-storage/csi-snapshotter:v8.5.0` |
| `registrar` | CSI node-driver-registrar sidecar | `registry.k8s.io/sig-storage/csi-node-driver-registrar:v2.17.0` |
| `snapshot-metadata` | CSI snapshot-metadata sidecar (RBD only) | `registry.k8s.io/sig-storage/csi-snapshot-metadata:v1.0.0` |
| `plugin` | Ceph-CSI driver plugin | `quay.io/cephcsi/cephcsi:v3.17.0` |
| `addons` | CSI-Addons sidecar | `quay.io/csiaddons/k8s-sidecar:v0.14.0` |
| `ex-snapshotter` | Extended snapshotter for CephFS volume groups (optional) | Not set by default |

> **Note**: You only need to specify the images you want to override. Any keys not provided in your ConfigMap will use the default images. The image references shown in the example ConfigMaps above are examples only; use the latest available or supported images for your environment.

## Configuration Levels

The operator supports image customization at two levels:

1. **Global (OperatorConfig)**: Apply custom images to all drivers managed by the operator
2. **Per-Driver**: Override images for a specific driver instance

When both are configured, the per-driver configuration takes precedence.

## Global Image Configuration

To apply custom images to all drivers, create a ConfigMap and reference it in the `OperatorConfig`:

### Step 1: Create the Image Set ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: custom-csi-images
  namespace: ceph-csi-operator-system
data:
  # Override only the images you need to customize
  plugin: "my-registry.example.com/cephcsi/cephcsi:v3.17.0"
  provisioner: "my-registry.example.com/sig-storage/csi-provisioner:v6.2.0"
  attacher: "my-registry.example.com/sig-storage/csi-attacher:v4.12.0"
  # ... add other images as needed
```

### Step 2: Reference in OperatorConfig

```yaml
apiVersion: csi.ceph.io/v1
kind: OperatorConfig
metadata:
  name: ceph-csi-operator-config
  namespace: ceph-csi-operator-system
spec:
  driverSpecDefaults:
    imageSet:
      name: custom-csi-images
```

## Per-Driver Image Configuration

To customize images for a specific driver, create a ConfigMap in the driver's namespace and reference it in the Driver CR:

### Step 1: Create the Image Set ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: rbd-custom-images
  namespace: ceph-csi-operator-system
data:
  plugin: "my-registry.example.com/cephcsi/cephcsi:v3.17.0-custom"
  snapshotter: "my-registry.example.com/sig-storage/csi-snapshotter:v8.5.0"
```

### Step 2: Reference in Driver CR

```yaml
apiVersion: csi.ceph.io/v1
kind: Driver
metadata:
  name: rbd.csi.ceph.com
  namespace: ceph-csi-operator-system
spec:
  imageSet:
    name: rbd-custom-images
```

## Example: Disconnected Deployment

For air-gapped or disconnected environments, you need to mirror all required images to your private registry:

### Step 1: Mirror Images to Private Registry

```bash
# Example using skopeo to mirror images
PRIVATE_REGISTRY="registry.internal.example.com"

# Mirror all required images
skopeo copy docker://registry.k8s.io/sig-storage/csi-provisioner:v6.2.0 \
  docker://${PRIVATE_REGISTRY}/sig-storage/csi-provisioner:v6.2.0

skopeo copy docker://registry.k8s.io/sig-storage/csi-attacher:v4.12.0 \
  docker://${PRIVATE_REGISTRY}/sig-storage/csi-attacher:v4.12.0

skopeo copy docker://registry.k8s.io/sig-storage/csi-resizer:v2.1.0 \
  docker://${PRIVATE_REGISTRY}/sig-storage/csi-resizer:v2.1.0

skopeo copy docker://registry.k8s.io/sig-storage/csi-snapshotter:v8.5.0 \
  docker://${PRIVATE_REGISTRY}/sig-storage/csi-snapshotter:v8.5.0

skopeo copy docker://registry.k8s.io/sig-storage/csi-node-driver-registrar:v2.17.0 \
  docker://${PRIVATE_REGISTRY}/sig-storage/csi-node-driver-registrar:v2.17.0

skopeo copy docker://registry.k8s.io/sig-storage/csi-snapshot-metadata:v1.0.0 \
  docker://${PRIVATE_REGISTRY}/sig-storage/csi-snapshot-metadata:v1.0.0

skopeo copy docker://quay.io/cephcsi/cephcsi:v3.17.0 \
  docker://${PRIVATE_REGISTRY}/cephcsi/cephcsi:v3.17.0

skopeo copy docker://quay.io/csiaddons/k8s-sidecar:v0.14.0 \
  docker://${PRIVATE_REGISTRY}/csiaddons/k8s-sidecar:v0.14.0
```

### Step 2: Create ConfigMap with Private Registry URLs

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: disconnected-images
  namespace: ceph-csi-operator-system
data:
  provisioner: "registry.internal.example.com/sig-storage/csi-provisioner:v6.2.0"
  attacher: "registry.internal.example.com/sig-storage/csi-attacher:v4.12.0"
  resizer: "registry.internal.example.com/sig-storage/csi-resizer:v2.1.0"
  snapshotter: "registry.internal.example.com/sig-storage/csi-snapshotter:v8.5.0"
  registrar: "registry.internal.example.com/sig-storage/csi-node-driver-registrar:v2.17.0"
  snapshot-metadata: "registry.internal.example.com/sig-storage/csi-snapshot-metadata:v1.0.0"
  plugin: "registry.internal.example.com/cephcsi/cephcsi:v3.17.0"
  addons: "registry.internal.example.com/csiaddons/k8s-sidecar:v0.14.0"
```

### Step 3: Apply to OperatorConfig

```yaml
apiVersion: csi.ceph.io/v1
kind: OperatorConfig
metadata:
  name: ceph-csi-operator-config
  namespace: ceph-csi-operator-system
spec:
  driverSpecDefaults:
    imageSet:
      name: disconnected-images
```

## Image Pull Secrets

If your private registry requires authentication, configure image pull secrets:

### Step 1: Create Image Pull Secret

```bash
kubectl create secret docker-registry private-registry-secret \
  --docker-server=registry.internal.example.com \
  --docker-username=<username> \
  --docker-password=<password> \
  --docker-email=<email> \
  -n ceph-csi-operator-system
```

### Step 2: Reference in Driver CR

```yaml
apiVersion: csi.ceph.io/v1
kind: Driver
metadata:
  name: rbd.csi.ceph.com
  namespace: ceph-csi-operator-system
spec:
  imageSet:
    name: disconnected-images
  controllerPlugin:
    imagePullSecrets:
      - name: private-registry-secret
  nodePlugin:
    imagePullSecrets:
      - name: private-registry-secret
```

## Verification

After applying your custom image configuration, verify that the correct images are being used:

```bash
# Check controller plugin deployment
kubectl get deployment -n ceph-csi-operator-system -o yaml | grep image:

# Check node plugin daemonset
kubectl get daemonset -n ceph-csi-operator-system -o yaml | grep image:
```

## Troubleshooting

### Images Not Updating

If your custom images are not being applied:

1. Verify the ConfigMap exists in the correct namespace:
   ```bash
   kubectl get configmap <configmap-name> -n <namespace>
   ```

2. Check the OperatorConfig or Driver CR references the correct ConfigMap name:
   ```bash
   kubectl get operatorconfig ceph-csi-operator-config -o yaml
   kubectl get driver <driver-name> -o yaml
   ```

3. Check operator logs for image loading errors:
   ```bash
   kubectl logs -n ceph-csi-operator-system deployment/ceph-csi-operator-controller-manager
   ```

### Image Pull Errors

If pods fail to pull images:

1. Verify the image URLs are correct and accessible from your cluster
2. Check if image pull secrets are properly configured
3. Verify network connectivity to your registry
4. Check pod events for specific error messages:
   ```bash
   kubectl describe pod <pod-name> -n ceph-csi-operator-system
   ```
