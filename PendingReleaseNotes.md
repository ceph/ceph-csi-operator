# v1.0.0 Pending Release Notes

## Breaking Changes

## Features

- Added `containerExtraArgs` field to `NodePluginSpec` and `ControllerPluginSpec` to allow passing custom arguments to CSI containers. The field accepts a map where the key is the container name (e.g., `csi-rbdplugin`, `csi-provisioner`, `driver-registrar`) and the value is a list of CLI arguments. This enables customization of container behavior without modifying default operator values. The field can be configured at both the OperatorConfig level (for defaults) and Driver level (for driver-specific overrides).
- Fencing can now be enabled in the ceph-csi-drivers Helm chart either globally for all drivers or on a per-driver basis.
- Added `enabled` field to log rotation configuration in the ceph-csi-drivers Helm chart. Users can now disable log rotation by setting `log.rotation.enabled: false` either globally in `operatorConfig.driverSpecDefaults` or per-driver in `drivers.<driver-type>`. This resolves Helm warnings when disabling file logging from parent charts.

## NOTE
