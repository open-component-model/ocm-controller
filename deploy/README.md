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

## Flux Install

We can also use Flux to install ocm-controller and all of its prerequisites
which are the certificates and cert-manager.

To see how it's done, take a look at the script under [flux/script.sh](./flux/script.sh).