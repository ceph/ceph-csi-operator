# Ceph CSI Operator

[![Go Report Card](https://goreportcard.com/badge/github.com/ceph/ceph-csi-operator)](https://goreportcard.com/report/github.com/ceph/ceph-csi-operator)
![License](https://img.shields.io/github/license/ceph/ceph-csi-operator)
[![Mergify Status](https://img.shields.io/endpoint.svg?url=https://api.mergify.com/v1/badges/ceph/ceph-csi-operator&style=flat)](https://mergify.com)
![Version](https://img.shields.io/github/v/release/ceph/ceph-csi-operator)

## Table of Contents

- [Ceph CSI Operator](#ceph-csi-operator)
  - [Table of Contents](#table-of-contents)
  - [Overview](#overview)
  - [Quick Start](#quick-start)
  - [Installation](#installation)
  - [Usage](#usage)
  - [Contributing](#contributing)
    - [Reporting Issues](#reporting-issues)
    - [Pull Requests](#pull-requests)
  - [License](#license)
  - [Contact](#contact)

## Overview

The Ceph CSI Operator provides native management interfaces for [Ceph-CSI drivers (CephFS, RBD, and NFS)](https://github.com/ceph/ceph-csi) for Kubernetes based environments. The operator automates the deployment, configuration, and management of these drivers using new Kubernetes APIs defined as a set of Custom Resource Definitions (CRDs).

## Quick Start

For those eager to get started quickly, follow the steps outlined in the [Quick Start Guide](docs/quick-start.md).

## Installation

For detailed installation instructions, including methods using Helm and manual deployment, please visit the [Installation Guide](docs/installation.md).

## Usage

To learn how to use the Ceph CSI Operator, refer to the [Usage Guide](docs/usage.md).

## Contributing

We welcome contributions to the Ceph CSI Operator! Please follow [development-guide](docs/development-guide.md)
and [coding style guidelines](docs/coding.md) if you are interested to contribute to this repo.

### Reporting Issues

If you encounter a problem, please [open an issue](https://github.com/ceph/ceph-csi-operator/issues) on GitHub. Be sure to fill the template when opening an issue.

### Pull Requests

We encourage you to submit pull requests for bug fixes, improvements, or new features. Please ensure that your code adheres to our coding standards and includes tests where applicable.

## License

The Ceph CSI Operator is licensed under the Apache License 2.0. See the [LICENSE](LICENSE) file for more details.

## Contact

Please use the following to reach members of the community:

- Slack: Join the
  [#ceph-csi](https://ceph-storage.slack.com/archives/C05522L7P60) channel
  on the [ceph Slack](https://ceph-storage.slack.com) to discuss anything
  related to this project. You can join the Slack by this
  [invite link](https://bit.ly/ceph-slack-invite)
