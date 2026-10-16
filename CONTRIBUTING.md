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

To test the controller locally you need to build it first with `make`.

There are two ways to test a controller. In-cluster or running it locally. In either cases a cluster is required to run
the manager first. Using kind, create a cluster with:

```
kind create cluster --name controller-test
```

The controller watches Flux source/kustomize/helm resources, so install Flux into the cluster first:

```
flux install --components=source-controller,helm-controller,kustomize-controller
```

If in-cluster is the preferred way to test the controller, build the image and load it into kind directly
(no local registry needed):

```
make docker-build TAG=dev
kind load docker-image ghcr.io/open-component-model/ocm-controller:dev --name controller-test
```

Install the controller with the Helm chart in `deploy/`, pointing the manager image at the tag you just loaded.
`pullPolicy=Never` forces kind to use the loaded image instead of pulling from the registry:

```
helm upgrade --install ocm-controller ./deploy -n ocm-system --create-namespace \
  --set manager.image.tag=dev \
  --set manager.image.pullPolicy=Never
```

The chart also deploys an in-cluster OCI registry. Its TLS certificate is provisioned by cert-manager when
`tlsCert.generateTlsCert=true`; if cert-manager is not installed, disable registry TLS with
`--set registry.tls.enabled=false`.

Per-controller concurrency can be set via `manager.concurrency`, e.g. `--set manager.concurrency.resource=3`.

If it's enough to run the manager locally, you don't need a local registry. Simply run:

```
./bin/manager -zap-log-level 4
```
