# Helm Chart Resources

This document describes the Helm Chart resource type and it's usage.

## From OCM to controller resource

Support for Helm chart based resources is made possible through OCM. Describing a Helm Chart based resource through OCM
component version might look something like this:

```yaml
components:
- name: github.com/open-component-model/helm-test
  version: "v1.0.0"
  provider:
    name: ocm.software
  resources:
  - name: charts
    type: helmChart
    version: 6.3.5
    input:
      type: helm
      version: 6.3.5
      path: charts/podinfo-6.3.5.tgz
```

This resource then can be defined for the controller like this:

```yaml
apiVersion: delivery.ocm.software/v1alpha1
kind: Resource
metadata:
  name: ocm-with-helm-deployment
  namespace: ocm-system
spec:
  interval: 10m
  sourceRef:
    kind: ComponentVersion
    name: ocm-with-helm
    namespace: ocm-system
    resourceRef:
      name: charts
      version: 6.3.5
      extraIdentity:
        helmChart: podinfo
```

Notice the `helmChart` extraIdentity field. This is necessary so the controller knows that this resource is a helm chart.
The OCM CLI at the time of this writing, sets the mimetype of this resource the ociImage. Therefor, the controller is
unable to detect that the resource which we are targeting is a helm chart.

This extra information is also used during constructing the repository name. For the reason read [](#gotchas).

## Local Registry Rewrite

Once this resource is fetched, we create a helm based oci repository as defined [here](https://helm.sh/docs/topics/registries/) in our
internal registry. This is from where the Flux resources will fetch and deploy the helm chart. No authentication is
necessary at this step as our internal registry runs on https and isn't accessible from the outside.

## Gotchas

We are using HelmRepositories and HelmReleases to install Helm Charts through Flux into a cluster. This happens with the
`FluxDeployer` object. Flux looks into an OCI registry for a chart by constructing a URL like this:
`oci://repo:port/repository/chart-name`. Normally, we don't include anything else into the OCI repository so this lookup
fails because we don't have the `chart-name` post-fix.

To that end, we made modifications to put the chart name into the repository name in case the resource of type `helmChart`.
The name is taken from the extraIdentity field defines when declaring a Resource, Localization or Configuration object.
