# Add support for TLS certificates for CSI Addons communication

The Kubernetes CSI Addons project currently connects the CSI Addons Manager to the add-on sidecar without TLS in place.
We propose a introduction of an arugment which enabled should mount the TLS certificates into the sidecar container.
The operator is only responsible for propagation of the certificates.

## Problem Statement

CSI addons sidecar which is being deployed here needs certificates. Currently we have no argument to enable this propagation of certificates.

## Proposed Solution

### Introduce a new argument for TLS

We introduce a new argument in the commands to enable TLS. It is diabled by default. But if this is enabled the deployer is expected to have mounted a secret that contains the required certificates. We will essentially need a certifiate that is compatible with the hostname that the manager will be issuing network calls using it.

### Operator changes

The Ceph CSI Operator when deploying the sidecar for CSI Addons should will the secret provided in the OperatorConfig in `/etc/tls` folder. The infomration about the secret should be provided via OperatorConfig which in then will be used during sidecar creation. The secret contains information the certficate that is going to be used by the CSI Addons sidecar.

## Guide to the operator incharge of creating certificates (Optional)

Based on the type of networking that is to be used. Hostnames used to connect to the Sidecar endpoint should be set in the certificate accordingly.

## Changes proposed to OperatorConfig

```
// New struct proposed
type TLSConfiguration struct {
    certificateSecretname string
    cretificateSecretNamespace string
    enableTLS bool
}

// OperatorConfigSpec defines the desired state of OperatorConfig
type OperatorConfigSpec struct {
	//+kubebuilder:validation:Optional
	Log *OperatorLogSpec `json:"log,omitempty"`

	// Allow overwrite of hardcoded defaults for any driver managed by this operator
	//+kubebuilder:validation:Optional
	DriverSpecDefaults *DriverSpec `json:"driverSpecDefaults,omitempty"`
    TLSConfiguration *TLSConfiguration // Conigure TLS certificates for CSI addons
}
```
