# Official Helm Charts for ocm-controller

## Installation

We are using ghcr.io's OCI registry for publishing helm charts.

To install it, simply run:

```
helm upgrade -i --wait --create-namespace -n ocm-system ocm-controller \
  oci://ghcr.io/open-component-model/helm/ocm-controller --version <VERSION>
```

## Configuration

The project is using plain Helm Values files for configuration options.
Check out the default values for the chart [here](https://raw.githubusercontent.com/open-component-model/ocm-controller/main/ocm-controller/values.yaml).

### Component scan concurrency

Set the maximum number of different `ComponentVersion` objects reconciled at once:

```yaml
manager:
  concurrency: 3
```

`manager.concurrency` sets the `--concurrency` flag and maps
to controller-runtime's `MaxConcurrentReconciles` for the ComponentVersion
controller. It defaults to `1` and must be a positive integer. The limit applies
per active controller manager, and the same object is still reconciled serially.

Each component's candidate versions are checked sequentially in descending
semantic version order. The highest version matching `spec.version.semver` that
passes all configured signatures is selected. Increasing component concurrency
allows independent component scans to overlap and can increase Artifactory
traffic, CPU, and memory use.

## Flux Install

We can also use Flux to install ocm-controller and all of its prerequisites
which are the certificates and cert-manager.

To see how it's done, take a look at the script under [flux/script.sh](./flux/script.sh).
