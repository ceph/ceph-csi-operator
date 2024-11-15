# Add support for TLS certificates for CSI Addons communication

The Kubernetes CSI Addons project currently connects the CSI Addons Manager to the add-on sidecar without TLS in place.
We propose a introduction of an arugment which enabled should mount the TLS certificates into the sidecar container.
The operator is only responsible for propagation of the certificates.

## Problem Statement

CSI addons sidecar which is being deployed here needs certificates. Currently we have no argument to enable this propagation of certificates.

## Proposed Solution

###  Introduce a new argument for TLS

We introduce a new argument in the commands to enable TLS. It is diabled by default. But if this is enabled the deployer is expected to have mounted a secret that contains the required certificates. We will essentially need a certifiate that is compatible with the hostname that the manager will be issuing network calls using it. 


### Operator changes

The Ceph CSI Operator is only responsible for taking in the information mounted to it and project those as volumes in the CSI Addons sidecar. The deployer of CSI Addons should create these certificates and mount it at `/etc/tls/tls.crt` and `/etc/tls/tls.key`. We keep this hardcoded to reduce the number of new arguments introduced. The logger should provide enough information if the user is not mounting this correctly for easy debugging.

## Guide to the deployeer for handling certificates

Since we use host networking the certificates should have these IP addresses as valid Subject names.
