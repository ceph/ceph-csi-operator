
# ClientProfileReplication

The ClientProfileReplication CR defines the replication destination mapping
for backup and disaster recovery scenarios. It enables the CSI driver to determine the
destination volume identifiers when client profile is different across cluster and pools
have different IDs across mirrored Ceph clusters.

## Purpose
- Provides destination client profile and pool mapping for the `GetReplicationDestinationInfo` RPC
- Enables DR/Backup orchestrators to discover correct destination volume IDs during failover
- ClientProfileReplication replaces the replication destination discovery function of ClientProfileMapping. However, ClientProfileMapping must be retained as long as PersistentVolumes with stale cluster IDs exist in the cluster. Removing ClientProfileMapping prematurely will break all CSI operations on those volumes. Over time, as old PVs are deleted and replaced with PVs created through GetReplicationDestinationInfo (which have correct volume handles), ClientProfileMapping becomes unnecessary and can be safely removed.

## Design Rationale

1. **Pool name mapping**: Pool names are consistent across mirrored Ceph clusters,
   while pool IDs can differ. Mapping by name is more intuitive and maintainable.
   **Important**: Pool names must remain constant across clusters. If pools are
   renamed on either cluster, the corresponding ClientProfileReplication CR must
   be updated to reflect the new pool names.

2. **Single destination**: The `GetReplicationDestinationInfo` RPC cannot receive
   selection parameters, therefore only one destination per local client profile
   is supported.

3. **Operator translation**: The operator translates this CR into the
   `replicationDestination` field in the ceph-csi-config ConfigMap, which the CSI
   driver reads at runtime.

## Constraints and Validation

1. **Unique active client profiles**: Only one ClientProfileReplication CR may be in
   `Ready` state for a given `localClientProfile`. If multiple CRs reference the same
   `localClientProfile`:
   - The oldest CR (by creation timestamp) is accepted and marked `Ready`
   - All other CRs are rejected and marked `Rejected` with a descriptive message

2. **Client profile dependency**: A ClientProfileReplication CR requires a
   corresponding ClientProfile CR with a matching name.

   - **Missing ClientProfile**: If a ClientProfileReplication references a
     `localClientProfile` that doesn't exist, the CR is marked `Rejected` with message:
     `"rejected: ClientProfile '<name>' not found"`

   - **Deletion protection**: The ClientProfile controller adds a finalizer to
     ClientProfile CR. If deletion is attempted, the finalizer blocks it until all referencing
     ClientProfileReplication CRs are deleted first. This prevents breaking
     replication configurations.

## API Definition

```go
// ClientProfileReplicationSpec defines the desired replication destination mapping
type ClientProfileReplicationSpec struct {
    // LocalClientProfile is the name of the local ClientProfile CR
    // +kubebuilder:validation:Required
    LocalClientProfile string `json:"localClientProfile"`
    
    // RemoteClientProfile is the name of the remote cluster's client profile
    // +kubebuilder:validation:Required
    RemoteClientProfile string `json:"remoteClientProfile"`
    
    // RBD contains RBD-specific replication configuration
    // +optional
    RBD *RBDReplicationSpec `json:"rbd,omitempty"`
}

// RBDReplicationSpec defines RBD-specific replication configuration
type RBDReplicationSpec struct {
    // PoolMapping maps local pool names to remote pool IDs
    // +optional
    PoolMapping []PoolMappingSpec `json:"poolMapping,omitempty"`
}

// PoolMappingSpec defines the mapping for a single pool
type PoolMappingSpec struct {
    // Name is the pool name (must be consistent across clusters)
    // +kubebuilder:validation:Required
    Name string `json:"name"`
    
    // RemoteID is the pool ID on the remote cluster
    // +kubebuilder:validation:Required
    RemoteID string `json:"remoteID"`
}

// ClientProfileReplicationStatus defines the observed state
type ClientProfileReplicationStatus struct {
    // Phase indicates the current state of this CR
    // +optional
    Phase string `json:"phase,omitempty"`
    
    // Message provides human-readable details about the current phase
    // +optional
    Message string `json:"message,omitempty"`
}

// Phase constants
const (
    // ClientProfileReplicationPhaseReady indicates the CR is accepted and active
    ClientProfileReplicationPhaseReady = "Ready"
    
    // ClientProfileReplicationPhaseRejected indicates the CR is rejected (conflict)
    ClientProfileReplicationPhaseRejected = "Rejected"
    
    // ClientProfileReplicationPhasePending indicates validation is in progress
    ClientProfileReplicationPhasePending = "Pending"
)

// ClientProfileStatus defines the observed state
type ClientProfileStatus struct {
    // Phase indicates the current state of this CR
    // +optional
    Phase string `json:"phase,omitempty"`
    
    // Message provides human-readable details about the current phase
    // +optional
    Message string `json:"message,omitempty"`
}

// Phase constants for ClientProfile
const (
    // ClientProfilePhaseReady indicates the CR is reconciled successfully
    ClientProfilePhaseReady = "Ready"
    
    // ClientProfilePhaseFailed indicates reconciliation failed
    ClientProfilePhaseFailed = "Failed"
    
    // ClientProfilePhasePending indicates reconciliation is in progress
    ClientProfilePhasePending = "Pending"
)
```

## Controller Design

The design uses two controllers working together:

1. **ClientProfileReplication Controller** - Validates and accepts/rejects CRs
2. **ClientProfile Controller** - Consumes ready CRs to build ConfigMap

### Field Index Setup

To efficiently look up ClientProfileReplication CRs by the referenced ClientProfile,
create a field index during controller initialization:

```go
// Index ClientProfileReplication by localClientProfile field
indexFunc := func(obj client.Object) []string {
    cpr := obj.(*ClientProfileReplication)
    if cpr.Spec.LocalClientProfile != "" {
        return []string{cpr.Spec.LocalClientProfile}
    }
    return nil
}

mgr.GetFieldIndexer().IndexField(
    context.Background(),
    &ClientProfileReplication{},
    "spec.localClientProfile",
    indexFunc,
)
```

### ClientProfileReplication Controller

This controller validates ClientProfileReplication CRs and ensures only one CR per
`localClientProfile` is in Ready state.

**Reconciliation flow**:
1. Fetch the ClientProfileReplication CR being reconciled
2. **Validate ClientProfile exists**:
   ```go
   var clientProfile ClientProfile
   err := r.Get(ctx, types.NamespacedName{
       Name: cr.Spec.LocalClientProfile,
       Namespace: cr.Namespace,
   }, &clientProfile)
   ```
   - If not found, mark CR as `Rejected` with message:
     `"rejected: ClientProfile '<name>' not found"`
   - Update status and stop reconciliation (return)
3. Look up all CRs referencing the same `localClientProfile` using the field index:
   ```go
   var allForProfile ClientProfileReplicationList
   err := r.List(ctx, &allForProfile,
       client.InNamespace(cr.Namespace),
       client.MatchingFields{"spec.localClientProfile": cr.Spec.LocalClientProfile})
   ```
4. **Conflict detection**:
   - Sort the list by creation timestamp (oldest first)
   - The oldest CR wins and gets marked `Ready`
   - All other CRs are marked `Rejected` with message:
     `"rejected: another ClientProfileReplication '<winner-name>' is already active for localClientProfile '<profile-name>'"`
5. **Winner validation** (for the CR being accepted):
   - Mark status as `Ready` with message: `"accepted"`
6. Update the CR's status with the determined phase and message
7. Trigger reconciliation of all other CRs with the same `localClientProfile` to update their status

**Status updates**:
```go
// For the accepted/winner CR
status := ClientProfileReplicationStatus{
    Phase: ClientProfileReplicationPhaseReady,
    Message: "accepted",
}

// For rejected CRs
status := ClientProfileReplicationStatus{
    Phase: ClientProfileReplicationPhaseRejected,
    Message: fmt.Sprintf("rejected: another ClientProfileReplication '%s' is already active for localClientProfile '%s'",
        winnerName, localClientProfile),
}
```

**Watches**:
- ClientProfileReplication CRs (primary resource)
- ClientProfile CRs (triggers reconciliation of all ClientProfileReplication CRs that reference it)
  - When a ClientProfile is created, rejected CRs may become ready
  - When a ClientProfile is deleted, ready CRs must be rejected
- Triggers reconciliation of all CRs with the same `localClientProfile` when any one changes

### ClientProfile Controller

The ClientProfile controller reads ready ClientProfileReplication CRs and builds the ConfigMap.

**Reconciliation flow**:
1. Read the ClientProfile CR to get cluster configuration
2. Look up ClientProfileReplication CRs using the field index:
   ```go
   var replicationList ClientProfileReplicationList
   err := r.List(ctx, &replicationList,
       client.InNamespace(clientProfile.Namespace),
       client.MatchingFields{"spec.localClientProfile": clientProfile.Name})
   ```
3. **Filter for ready state**:
   - Filter the list to find CRs with `status.phase == ClientProfileReplicationPhaseReady`
   - Should only be 0 or 1 ready CRs (enforced by ClientProfileReplication controller)
   - If somehow multiple Ready CRs exist, log a warning and use the first one
4. Build ConfigMap entry:
   - Add cluster configuration from ClientProfile
   - If a Ready ClientProfileReplication found, add `replicationDestination` field
   - If no Ready CR found, omit `replicationDestination` (valid state)
5. Update the `config.json` key of the ceph-csi-config ConfigMap
6. Update ClientProfile status to reflect success

**Deletion handling**:
- Enhanced finalizer processing uses the index to check for referencing
  ClientProfileReplication CRs (any phase)
- If any exist, block deletion and update status with error:
  `"cannot delete: ClientProfileReplication CRs still reference this profile: [cr1, cr2]"`
- If none, proceed with normal cleanup

**Watches**:
- ClientProfile CRs (primary resource)
- ClientProfileReplication CRs (triggers reconciliation of referenced ClientProfile)

This design ensures:
- Clear separation of concerns: validation vs consumption
- Only one CR per `localClientProfile` can be Ready at a time
- Deterministic winner selection (oldest CR wins)
- ClientProfile controller only uses Ready CRs
- Single serialization point for ConfigMap updates (ClientProfile controller only)
- Deletion protection prevents breaking replication configurations

## Migration from ClientProfileMapping

### Upgrade Path for Existing Clusters

When upgrading from ClientProfileMapping to ClientProfileReplication on an existing
cluster with replicated volumes:

**Problem**: Existing PersistentVolumes contain volume handles with stale/destroyed
cluster IDs that were mapped via ClientProfileMapping. These PVs will continue to
work only as long as ClientProfileMapping exists.

**Migration Strategy**:

1. **Initial State** (before upgrade):
   - Cluster uses ClientProfileMapping for both cluster ID mapping and replication
     destination discovery
   - PVs contain volume handles with old cluster IDs (e.g., `cluster-a`)
   - ClientProfileMapping maps `cluster-a` → `cluster-d`

2. **Post-Upgrade State** (immediately after upgrade):
   - Deploy ClientProfileReplication CRs alongside existing ClientProfileMapping
   - **Keep both CRs active** - they serve different purposes now:
     - ClientProfileMapping: Required for CSI operations on old PVs with stale cluster IDs
     - ClientProfileReplication: Used by DR orchestrators for new failover operations
   - Ceph CSI ConfigMap contains both `clientProfileMapping` and `replicationDestination` fields

3. **Transition Period**:
   - New failovers use `GetReplicationDestinationInfo` RPC
   - New PVs created during failover have correct volume handles with current cluster IDs
   - Old PVs continue working via ClientProfileMapping
   - Gradual replacement: as applications failover/failback, old PVs are deleted and
     replaced with new PVs containing correct volume handles

4. **End State** (later, when all old PVs are gone):
   - All PVs now have correct volume handles
   - ClientProfileMapping can be safely deleted
   - Only ClientProfileReplication remains

**Timeline**: The migration is complete when administrators verify that no PVs remain
with old cluster IDs in their volume handles. This can take weeks to months depending
on application lifecycle and DR testing frequency.

### Co-existence Behavior

When both ClientProfileMapping and ClientProfileReplication exist for the same cluster:

**Operational Behavior**:
- **CSI volume operations** (create, delete, attach, mount) on old PVs use
  `clientProfileMapping` to resolve stale cluster IDs
- **GetReplicationDestinationInfo RPC** uses `replicationDestination` to map
  source → destination for new failover operations
- Both fields are independent and non-conflicting
- Deleting ClientProfileMapping while old PVs exist will break those PVs permanently

**Controller Coordination**:
- ClientProfile controller reads both ClientProfileMapping and ClientProfileReplication CRs
- Builds ConfigMap with both `clientProfileMapping` and `replicationDestination` fields
- No conflict as they serve different purposes

## DR/Backup Consumption Model

### How DR Orchestrators Use ClientProfileReplication

DR/Backup orchestrators (e.g. Ramen) use the
`GetReplicationDestinationInfo` RPC to discover correct destination volume handles
during failover operations.

**Workflow**:

1. **Pre-Failover** (Primary cluster active):
   - Application runs on primary cluster with PVs
   - RBD mirroring replicates volumes to secondary cluster
   - Volume handles on primary: `0001-000f-primary-cluster-0000000000000001-<uuid>`

2. **Disaster Event**:
   - Primary cluster goes down
   - DR orchestrator initiates failover to secondary cluster

3. **Destination Discovery** (on secondary cluster):
   - DR orchestrator reads backed-up PV manifests (from Velero/OADP)
   - Extracts source volume handles from PV specs
   - Calls `GetReplicationDestinationInfo` RPC with source volume handle:
     ```go
     req := &GetReplicationDestinationInfoRequest{
         ReplicationSource: &ReplicationSource{
             Type: &ReplicationSource_Volume{
                 Volume: &VolumeSource{
                     VolumeId: "0001-000f-primary-cluster-0000000000000001-<uuid>",
                 },
             },
         },
         Secrets: map[string]string{...},
     }
     ```
   - CSI driver reads `replicationDestination` from ConfigMap
   - Maps source volume ID to destination:
     - Input: `0001-000f-primary-cluster-0000000000000001-<uuid>`
     - Output: `0001-0011-secondary-cluster-0000000000000005-<uuid>`
     - (pool ID changed 1→5 based on ClientProfileReplication mapping)

4. **PV Recreation** (on secondary cluster):
   - DR orchestrator creates new PV manifest with destination volume handle
   - PVC binds to new PV
   - Application starts using the mirrored volume on secondary cluster
   - **Volume handle now has correct cluster ID** (`secondary-cluster`)

5. **Post-Failover State**:
   - New PVs have correct volume handles for secondary cluster
   - No ClientProfileMapping needed for these PVs
   - Future operations (attach, mount, delete) work directly without mapping

### Volume Group Support

For applications using VolumeGroups (consistency groups), the RPC handles entire groups:

```go
req := &GetReplicationDestinationInfoRequest{
    ReplicationSource: &ReplicationSource{
        Type: &ReplicationSource_Volumegroup{
            Volumegroup: &VolumeGroupSource{
                VolumeGroupId: "0001-000f-primary-cluster-0000000000000001-<group-uuid>",
            },
        },
    },
}
```

Response includes:
- Destination volume group ID
- Map of all source volume IDs → destination volume IDs

This enables DR orchestrators to maintain consistency group relationships during failover.

## ConfigMap Translation

The operator translates the CR into the `config.json` key of the ceph-csi-config ConfigMap:

```json
[{
  "clusterID": "primary-cluster",
  "monitors": ["10.0.0.1:6789"],
  "replicationDestination": {
    "remoteClusterID": "secondary-cluster",
    "rbd": {
      "remotePoolMapping": {
        "rbd": {
          "poolID": "5"
        },
        "replicapool": {
          "poolID": "6"
        }
      }
    }
  }
}]
```

## Example: Bidirectional Replication

For disaster recovery with failback capability, create mappings in both directions:

```yaml
---
# Primary → Secondary (for initial failover)
kind: ClientProfileReplication
apiVersion: csi.ceph.io/v1
metadata:
  name: primary-to-secondary
  namespace: <operator-namespace>
spec:
  localClientProfile: primary-cluster
  remoteClientProfile: secondary-cluster
  rbd:
    poolMapping:
      # Pool "rbd" has ID 1 on primary, ID 5 on secondary
      - name: rbd
        remoteID: "5"
      # Pool "replicapool" has ID 2 on primary, ID 6 on secondary
      - name: replicapool
        remoteID: "6"
---
# Secondary → Primary (for failback)
kind: ClientProfileReplication
apiVersion: csi.ceph.io/v1
metadata:
  name: secondary-to-primary
  namespace: <operator-namespace>
spec:
  localClientProfile: secondary-cluster
  remoteClientProfile: primary-cluster
  rbd:
    poolMapping:
      # Pool "rbd" has ID 5 on secondary, ID 1 on primary
      - name: rbd
        remoteID: "1"
      # Pool "replicapool" has ID 6 on secondary, ID 2 on primary
      - name: replicapool
        remoteID: "2"
```
