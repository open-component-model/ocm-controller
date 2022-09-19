# OCM Controller Design

- [1 Overview](#overview)
- [1.1 Architecture ](#11-architecture)
- [1.2 Concepts](#12-concepts)
- [1.2.1 Snapshot](#121-snapshot)
- [1.2.2 Source](#121-source)
- [1.2.3 Action](#123-action)
- [1.3 Custom Resource Definitions](#13-custom-resource-definitions)
- [1.3.1 OCM Component CR](#131-ocmcomponent-cr)
- [1.3.2 Source CR](#132-source-cr)
- [1.3.3 Action CR](#133-action-cr)
- [1.3.4 OCMResource CR](#134-ocmresource-cr)
- [2 Controllers](#2-controllers)
- [2.1 OCM Core Controller](#21-ocm-core-controller)
- [3 Sample Workflow](#3-sample-workflow)
- [3.1 Overview](#31-overview)
- [3.2 Walkthrough](#32-walkthrough)

## 1 Overview

The `ocm-controller` is a suite of Kubernetes controllers that manage the deployment of software delivered using the Open Component Model (OCM). It provides a pluggable framework enabling user supplied last-mile transformation operations (such as localization and customization) to be applied and combined freely.

## 1.1 Architecture

The design of `ocm-controller` takes inspiration from both the Flux and Cluster API projects. It is made of up the `ocm-core-controller` and provider-implemented controllers (providers). The `ocm-core-controller` must be installed. Providers may be combined freely. The `ocm-core-controller` provides storage in the form of an OCI registry which is used to store intermediate component transformations (snapshots) produced by provider controllers. The `ocm-core-controller` assumes responsibility for these snapshots by performing verification and providing attestation regarding snapshot providence.

Provider controllers must meet the "provider contract" in order to integrate with the `ocm-controller`:

The contract required for Source provider resources is:

- the status must contain a boolean field ready
- the status must contain a snapshot field with the name of the snapshot OCI artifact
- the status must contain a digest field with the digest of the snapshot OCI artifact

The contract required for Action provider resources is:

- the status must contain a boolean field ready
- the status may contain a snapshot field with the name of the snapshot OCI artifact
- the status may contain a digest field with the digest of the snapshot OCI artifact

A provider type that meets the contract for Source or Action is called a sub-type.

## 1.2 Concepts

### 1.2.1 Snapshot

A snapshot is a gzip compressed tar archive of a component resource that is stored in layer-0 of an OCI Artifact. Snapshots are stored

The OCI artifact should be tagged using the following tagging schema:

`<OCMComponent name>/<Source or Action name>-<Provider Ref name>:<Unix timestamp>`

### 1.2.2 Source

A "Source" makes an OCM resource available for consumption by downstream "Actions". "Sources" may have access to external storage.

### 1.2.3 Action

An "Action" consumes a snapshot provided by a "Source" or another "Action". "Actions" may produce snapshots or may perform some other work such as creating a Pull Request or generating Flux objects.

## 1.3 Custom Resource Definitions

The `ocm-controller` relies on a number of CustomResourceDefinitions in order to extend the Kubernetes API and provide OCM-specific functionality. These are currently implemented as part of the `ocm-core-controller`.

### 1.3.1 OCMComponent CR

(TODO: should this properly be renamed OCMComponentVersion?)

An "OCMComponent" is the declarative spec for a particular OCMComponent. It provides details about the component version, the OCM registry and the verification key that can be used by the `ocm-core-controller` to retrieve a Component Version from an OCM repository.


`OCMComponent` is a namespaced resource with the following fields:
```yaml
apiVersion: delivery.ocm.software/v1alpha1
kind: OCMComponent
metadata:
  name: weave-gitops
  namespace: weave-gitops-system
spec:
  interval: 10m
  name: github.com/weaveworks/weave-gitops
  version: v0.9.x
  repository:
    url: ghcr.io/weaveworks
    secretRef:
      name: oci-creds
  verify:
    secretRef:
      name: my-public-key
status:
  componentDescriptor: weave-gitops-system/github-com-weaveworks-wge-v0-9-4
  deployPackage: weave-gitops-system/github-com-weaveworks-wge-v0-9-4-deploy-config
  verified: true
```

### 1.3.2 Source CR

A "Source" is a declarative spec for an Kubernetes resource that will generate a snapshot of an OCM resource which is persisted in an OCI-registry provided by the `ocm-core-controller`. Sources delegate the retrieval and pushing of a source to the provider. Sources are responsible for verifying and attesting the snapshots produced by providers, enabling downstream transformers (i.e. Actions) to trust the snapshots they consume. A "Source" is considered logically separate from an "Action" because it's sole purpose is to procure resources and provide snapshots. For example, "Sources" may be granted access to an OCM registry to which, from a security architecture perspective, it is not desirable to grant access to "Action".


Source is a namespaced resource with the following fields:

```yaml
apiVersion: delivery.ocm.software/v1alpha1
kind: Source
metadata:
  name: ui-server
  namespace: wge-system
spec:
  componentRef:
    name: weave-gitops
    namespace: wge-system
  providerRef:
    apiVersion: delivery.ocm.software/v1alpha1
    kind: OCMResource
    name: wge-ui-server
status:
  ready: true
  snapshot: string
  digest: string
```
### 1.3.3 Action CR

An "Action" is a declarative spec for an Kubernetes resource that will read a snapshot and perform an action, optionally generating a subsequent snapshot that is persisted by the `ocm-core-controller`. Actions delegate the retrieval, processing and pushing of snapshots to the provider. Actions are responsible for verifying and attesting the snapshots produced by providers, enabling downstream transformers (i.e. Actions) to trust the snapshots they consume. An `Action` is not required to both consume a `Source` and produce a snapshot but it should do at least one of these operations.

`Action` is a namespaced resource with the following fields:
```yaml
apiVersion: delivery.ocm.software/v1alpha1
kind: Action
metadata:
  name: localize
  namespace: wge-system
spec:
  componentRef:
    name: wge
    namespace: wge-admin
  snapshotSourceRef:
    kind: Source
    name: wge-admin-source
  providerRef:
    apiVersion: delivery.ocm.software/v1alpha1
    kind: Localization
    name: wge-localizer
status:
  ready: true
  snapshot: string
```

### 1.3.4 OCMResource CR

An "OCMResource" is a Source-subtype implementation that can retrieve a Resource from an OCMRepository and make it accessible for consumption for downstream Action-subtypes.

`OCMResource` is a namespaced resource with the following fields:

```yaml
apiVersion: delivery.ocm.software/v1alpha1
kind: OCMResource
metadata:
  name: wge-ui-server
  namespace: wge-system
spec:
  resource: wge-ui-server
status:
  ready: true
  snapshot: string
  digest: string
```
## 2 Controllers

### 2.1 OCM Core Controller

The ocm-controller is responsible for managing OCM specific API resources and storage for OCM transformations.

The ocm-controller should reconcile OCMComponent resources by fetching the associated ComponentDescriptor from the OCM repository and apply the resource to the cluster. This enables other controllers to query the `ComponentDescriptor` via the Kubernetes API.

For a given OCMComponent, the `ocm-controller` should also reconcile the Deploy Package specified by the flux media type. This may be applied as a dedicated API type or as a ConfigMap. When both have been reconciled the `ocm-controller` should update the status field of the `OCMComponent` with the namespaced name of the ComponentDescriptor and DeployPackage. The `OCMComponent` controller should also handle verification of components using a public key supplied via the `spec.verify.secretRef` field.

The `ocm-controller` is responsible for creating an in-cluster OCI registry which will be used as storage for snapshots. The registry should be created on startup with a new repository created for each OCMComponent. The registry should be accessed via port "5001" on  the `ocm-core-controller` which is exposed within the cluster as a Kubernetes service. The service endpoint can be used to when deploying providers in order to configure access from to the OCI storage.


## 3 Sample Workflow

### 3.1 Overview

![](https://i.imgur.com/9n7paGD.png)

The following resources would be checked-in to a repository managed by flux. (We assume that all controllers are already running.)

The demo illustrates how the component described [here](https://github.com/open-component-model/demo/tree/main/components/fluxdemo) can be deployed using Flux.

The component is a podinfo application and contains three resources:
- the podinfo container image (type: `ociImage`)
- kubernetes manifests to deploy the podinfo service (type: `kustomize.ocm.fluxcd.io`)
- a package file describing configuration parameters for ocm instance deployment (type: `package.ocm.fluxcd.io`)

The component is built and transferred to the consumer repository. A `OCMComponent` is used to store the `ComponentDescriptor` in the Kubernetes API. The package file is also stored, either as a ConfigMap or a new custom resource.

Sources and Actions can subsequently refer to the`OCMComponent` in order to read the `ComponentDescriptor` and deploy package.

### 3.2 Walkthrough

First of all we create the `OCMComponent`. This will retrieve the Component Descriptor and Deploy Package from the OCM repository and store in the Kubernetes API:

```yaml
apiVersion: delivery.ocm.software/v1alpha1
kind: OCMComponent
metadata:
  name: weave-gitops
  namespace: weave-gitops-system
spec:
  interval: 10m
  name: github.com/weaveworks/weave-gitops
  version: v0.9.x
  repository:
    url: ghcr.io/weaveworks
    secretRef:
      name: oci-creds
  verify:
    secretRef:
      name: my-public-key
```

Next we create a `Source` that references the `OCMComponent` and source sub-type`OCMResource` which will extract the desired resource and store the snapshot:

```yaml
apiVersion: delivery.ocm.software/v1alpha1
kind: Source
metadata:
  name: ui-server
  namespace: weave-gitops-system
spec:
  componentRef:
    name: weave-gitops
    namespace: weave-gitops-system
  providerRef:
    apiVersion: delivery.ocm.software/v1alpha1
    kind: OCMResource
    name: wge-ui-server
```

```yaml
apiVersion: delivery.ocm.software/v1alpha1
kind: OCMResource
metadata:
  name: wge-ui-server
  namespace: weave-gitops-system
spec:
  resource: wge-ui-server
```

Create an `Action` to consume the snapshot localize the `Source`:

```yaml
apiVersion: delivery.ocm.software/v1alpha1
kind: Action
metadata:
  name: localizer
  namespace: weave-gitops-system
spec:
  componentRef:
    name: wge
    namespace: wge-admin
  snapshotSourceRef:
    kind: Source
    name: wge-admin-source
  providerRef:
    apiVersion: delivery.ocm.software/v1alpha1
    kind: Localization
    name: wge-localizer
```

Here we create the provider for the `Action`, :

```yaml
apiVersion: delivery.ocm.software/v1alpha1
kind: Localization
metadata:
  name: wge-localizer
  namespace: weave-gitops-system
spec: {}
```

Next we provide the configuration `Action`, notice we use the `localizer` action as the `snapshotSourceRef`:

```yaml
apiVersion: delivery.ocm.software/v1alpha1
kind: Action
metadata:
  name: config
  namespace: weave-gitops-system
spec:
  componentRef:
    name: wge
    namespace: weave-gitops-system
  snapshotSourceRef:
    kind: Action
    name: localizer
  providerRef:
    apiVersion: delivery.ocm.software/v1alpha1
    kind: Configurator
    name: wge-config
---
apiVersion: delivery.ocm.software/v1alpha1
kind: Configurator
metadata:
  name: wge-config
  namespace: weave-gitops-system
spec:
  values:
  - debug: false
    rate_limit: false
    allowed_domains:
    - example.io
```

Finally we create an action for our Flux deployer which will generate a Flux `OCIRepository` and `Kustomization` allowing us to deploy the contents of a snapshot:

```yaml
apiVersion: delivery.ocm.software/v1alpha1
kind: Action
metadata:
  name: deployer
  namespace: weave-gitops-system
spec:
  componentRef:
    name: wge
    namespace: weave-gitops-system
  sourceRef:
    kind: Action
    name: config
  providerRef:
    apiVersion: delivery.ocm.software/v1alpha1
    kind: KustomizationTemplate
    name: wge-deploy
```

Finally we create the `KustomizationTemplate` sub-type resource:

```yaml
apiVersion: delivery.ocm.software/v1alpha1
kind: KustomizationTemplate
metadata:
  name: wge-deploy
spec:
  template:
    interval: 10m
    path: ./kubernetes
    targetNamespace: my-application-ns
```
