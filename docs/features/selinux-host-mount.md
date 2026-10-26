# SELinux host mount for node plugins

On nodes where SELinux is in enforcing mode (for example OpenShift/RHCOS, or
RHEL and Fedora hosts with SELinux enforcing), the node plugin needs access to
the host's SELinux configuration. Enable it with the `enableSeLinuxHostMount`
field of the node plugin spec. It defaults to `false`.

## What the setting does

When `nodePlugin.enableSeLinuxHostMount` is `true`, the operator adds a
`hostPath` volume for `/etc/selinux` to the node plugin DaemonSet and mounts
it read-only at `/etc/selinux` in the `csi-<driver type>plugin` container
(for example `csi-rbdplugin`). Nothing else changes. The setting applies to
every driver type.

## Why it is needed

The operator always creates the `CSIDriver` object with `seLinuxMount: true`.
For volumes where the kubelet applies the SELinux label at mount time (see the
Kubernetes `SELinuxMountReadWriteOncePod` and `SELinuxMount` feature gates),
the kubelet then passes a `context=...` mount option to the node plugin
instead of relabeling the volume contents itself.

The `mount` command in the plugin container only honors `context=` when
libselinux reports SELinux as enabled, and libselinux needs
`/etc/selinux/config` for that. Without the host mount, that file is missing
in the container, so `mount` silently drops the `context=` option and still
succeeds. The volume is mounted without the expected label.

Typical symptoms for `Filesystem` volumes on SELinux-enforcing nodes:

- The volume root is labeled `system_u:object_r:unlabeled_t:s0`, and the
  mount options inside the application pod do not contain `context=`.
- Non-root or confined application pods (for example pods running under the
  OpenShift `restricted-v2` SCC) get `Permission denied` when writing to the
  volume, even though the file ownership and `fsGroup` look correct.
- The node audit log shows AVC denials such as
  `scontext=...:container_t:... tcontext=system_u:object_r:unlabeled_t:s0`.

See [ceph/ceph-csi#6679](https://github.com/ceph/ceph-csi/issues/6679) for a
detailed analysis.

## Configuration

To enable the host mount for all drivers, set it in the `OperatorConfig`
driver defaults:

```yaml
apiVersion: csi.ceph.io/v1
kind: OperatorConfig
metadata:
  name: ceph-csi-operator-config
  namespace: ceph-csi-operator-system
spec:
  driverSpecDefaults:
    nodePlugin:
      enableSeLinuxHostMount: true
```

To set it for a single driver, use the `Driver` CR. A value set on the
`Driver` takes precedence over the `OperatorConfig` default:

```yaml
apiVersion: csi.ceph.io/v1
kind: Driver
metadata:
  name: rbd.csi.ceph.com
  namespace: ceph-csi-operator-system
spec:
  nodePlugin:
    enableSeLinuxHostMount: true
```

With the `ceph-csi-drivers` Helm chart, set
`operatorConfig.driverSpecDefaults.nodePlugin.enableSeLinuxHostMount` or
`drivers.<driver>.nodePlugin.enableSeLinuxHostMount` to `true`.

## Applying the change to existing volumes

Changing the setting rolls out the node plugin DaemonSet. Volumes that are
already mounted keep their current mount options. They are mounted with the
`context=` option the next time they are staged on a node, for example after
all pods using the volume on that node have been deleted and recreated.

To verify, check the mount options from inside an application pod:

```console
kubectl exec <pod> -- grep <mount path> /proc/mounts
```

The output should include `context="system_u:object_r:container_file_t:..."`.
