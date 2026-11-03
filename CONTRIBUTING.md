# Contributions

Welcome to the OCM community!

We welcome many types of contributions.

For the general contribution process (fork-and-pull workflow, commit requirements, code of conduct, and more), see the
[central contributing guide](https://ocm.software/community/contributing/) on the project website.

## This repository

This controller has been created using [kubebuilder](https://book.kubebuilder.io/). As such, it follows standards and
practices outlined by kubebuilder.

To work on CRD definitions, after changes always run `make manifests && make generate`. The CRDs can be found under the
`api` folder.

### Testing the Controller

For in-cluster development, use the maintained Tilt workflow described in
[Local Testing](README.md#local-testing). It builds the controller image using
`tilt.dockerfile` (or `tilt.debug.dockerfile` when debugging) and reloads it as the
code changes:

```sh
make prime-test-cluster
flux install
tilt up
```

The cluster setup provisions the registry certificates required by the controller.
It requires kind, kubectl, Docker, Flux and Tilt, and installs the generated root
certificate into your local trust store.

Per-controller concurrency can be set via `manager.concurrency`, e.g.
`--set manager.concurrency.resource=3` when installing the Helm chart. See
[Controller concurrency](README.md#controller-concurrency) for the supported names.

To run the manager on your host instead, build it with `make` and run it against a
cluster with the required CRDs, RBAC, registry and certificates installed:

```sh
./bin/manager -zap-log-level 4
```

### Testing the Helm chart

Install the Helm unittest plugin, then run the chart checks:

```sh
helm plugin install https://github.com/helm-unittest/helm-unittest.git --version v1.2.1 --verify=false
make test-helm
```
