# Development Guide

## New to Go

Ceph-csi-operator is written in Go and if you are new to the language,
it is **highly** encouraged to:

* Take the [A Tour of Go](http://tour.golang.org/welcome/1) course.
* [Set up](https://golang.org/doc/code.html) Go development environment on your machine.
* Read [Effective Go](https://golang.org/doc/effective_go.html) for best practices.

## Development Workflow

### Workspace and repository setup

* [Download](https://golang.org/dl/) Go and
   [install](https://golang.org/doc/install) it on your system.
* Clone the ceph-csi-operator
  repo](<https://github.com/ceph/ceph-csi-operator>)
* Fork the [ceph-csi-operator repo](https://github.com/ceph/ceph-csi-operator) on Github.
* Add your fork as a git remote:

    ```console
    git remote add fork https://github.com/<your-github-username>/ceph-csi-operator
    ```

> Editors: Our favorite editor is vim with the [vim-go](https://github.com/fatih/vim-go)
> plugin, but there are many others like [vscode](https://github.com/Microsoft/vscode-go)

### Building ceph-csi-operator

To build ceph-csi-operator locally run:

```console
make
```

To build ceph-csi-operator in a container:

```console
make docker-build
```

The built binary will be present under `bin/` directory.

### Running ceph-csi-operator tests

Once the changes to the sources compile, it is good practice to run the tests that validate the style and other basics of the source code. Execute the unit tests (in the `*_test.go` files) and check the formatting of YAML files, MarkDown documents and shell scripts:

```console
make test
```

In addition to running tests locally, each Pull Request that is created will
trigger Continuous Integration tests.

### Code contribution workflow

ceph-csi-operator repository currently follows GitHub's
[Fork & Pull] (<https://help.github.com/articles/about-pull-requests/>) workflow
for code contributions.

Please read the [coding guidelines](coding.md) document before submitting a PR.

#### Certificate of Origin

By contributing to this project you agree to the Developer Certificate of Origin (DCO). This document was created by the Linux Kernel community and is a simple statement that you, as a contributor, have the legal right to make the contribution. See the [DCO](DCO) file for details.

Contributors sign-off that they adhere to these requirements by adding a Signed-off-by line to commit messages. For example:

```text
This is my commit message

More details on what this commit does

Signed-off-by: Random J Developer <random@developer.example.org>
```

If you have already made a commit and forgot to include the sign-off, you can amend your last commit to add the sign-off with the following command, which can then be force pushed.

```console
git commit --amend -s
```

We use a [DCO bot](https://github.com/apps/dco) to enforce the DCO on each pull request and branch commits.

#### Commit Messages

We follow a rough convention for commit messages that is designed to answer two questions: what changed and why? The subject line should feature the what and the body of the commit should describe the why.

```text
fix bug in configmap

fix clusterID bug in the configmap where the clusterID is
not set to the expected value.

Signed-off-by: Random J Developer <random@developer.example.org>
```

The format can be described more formally as follows:

```text
<subject of the change>
<BLANK LINE>
<paragraph(s) with reason/description>
<BLANK LINE>
<signed-off-by>
```

The first line is the subject and should be no longer than 70 characters, the second line is always blank, and other lines should be wrapped at 80 characters. This allows the message to be easier to read on GitHub as well as in various git tools.

Here is a short guide on how to work on a new patch. In this example, we will work on a patch called *hellopatch*:

1. Make sure you Fork's main branch is up to date

    ```console
    git fetch upstream main:main
    git checkout main
    git push
    ```

2. Create a new branch for your patch:

    ```console
    git checkout -b hellopatch
    ```

Do your work here and commit.

Run the test suite, which includes linting checks, static code check, and unit tests:

```console
make test
```

Once you are ready to push, you will type the following:

```console
git push hellopatch
```

**Creating A Pull Request:**
When you are satisfied with your changes, you will then need to go to your repo in GitHub.com and create a pull request for your branch. Automated tests will be run against the pull request. Your pull request will be reviewed and merged.

If you are planning on making a large set of changes or a major architectural change it is often desirable to first build a consensus in an issue discussion and/or create an initial design doc PR. Once the design has been agreed upon one or more PRs implementing the plan can be made.

A few labels interact with automation around the pull requests:

* ready-to-merge: This PR is ready to be merged and it doesn't need second review
* do-not-merge: DO NOT MERGE (Mergify will not merge this PR)
* ok-to-test: PR is ready for e2e testing.

**Review Process:**
Once your PR has been submitted for review the following criteria will need to be met before it will be merged:

When the criteria are met, a project maintainer can merge your changes into the project's main branch.

### Backport a Fix to a Release Branch

The flow for getting a fix into a release branch is:

1. Open a PR to merge the changes to main following the process outlined above.
2. Add the backport label to that PR such as `backport-to-release-vX.Y.Z`
3. After your PR is merged to main, the mergify bot will automatically open a PR with your commits backported to the release branch
4. If there are any conflicts you will need to resolve them by pulling the branch, resolving the conflicts and force push back the branch
5. After the CI is green, the bot will automatically merge the backport PR.
