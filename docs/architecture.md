## Architecture

This document explains the architecture of the OCM Kubernetes Controller Set (KCS). The purpose of the KCS is to enable the automated deployment of components using Kubernetes and Flux.

The following functions are provided as part of the KCS:

- Replication: replication of components from one OCM repository to another
- Signature Verification: verification of component signatures before resources are reconciled
- Resource Reconciliation: individual resources can be extracted from a component and reconciled to machines internal or external to the cluster
- Resource transformation: resource localization & configuration can be performed out of the box, with any other kind of modification supported via an extensible architecture
- Component extractions: multiple resources can be extracted from a component and transformed with a common set of user definable operations
- Git synchronization: resources extracted from a component can be pushed to a git repository

One of the central design decisions underpinning KCS is that resources should be composable. To this end we have introduced the concept of **Snapshots**; snapshots are immutable, Flux-compatible, single layer OCI images containing a single OCM resource. Snapshots are stored in an in-cluster registry and in addition to making component resources accessible for transformation, they also can be used as a caching mechanism to reduce unnecessary calls to the source OCM registry.

## Controllers

The KCS consists of the following controllers:
- OCM controller
- Replication controller
- Git sync controller

### OCM controller

The `ocm-controller` is responsible for the core work necessary to utilise resources from an `OCM` component in a Kubernetes cluster. This includes resolving `ComponentDescriptor` metadata for a particular component version, performing authentication to OCM repositories, retrieving artifacts from OCM repositories, making individual resources from the OCM component available within the cluster, performing localization and configuration.

Snapshots are used to pass resources between controllers and are stored in an in-cluster registry.

The `ocm-controller` consists of 5 sub-controllers:
- [Component Version Controller](#component-version-controller)
- [Resource Controller](#resource-controller)
- [Snapshot Controller](#snapshot-controller)
- [Localization Controller](#localization-controller)
- [Configuration Controller](#configuration-controller)
- [FluxDeployer Controller](#fluxdeployer-controller)

#### Component Version Controller

The Component Version controller reconciles component versions from an OCI repository by fetching the component descriptor and any referenced component descriptors. The component version controller will also verify signatures for all the public keys provided. The Component Version controller does not fetch any resources other than component descriptors. It is used by downstream controllers to access component descriptors and to attest the validity of component signatures. Downstream controllers can look up a component descriptor via the status field of the component version resource. 

```mermaid
sequenceDiagram
    User->>Kubernetes API: submit ComponentVersion CR
    Kubernetes API-->>Component Version Controller: Component Version Created Event
    Component Version Controller->>OCM Repository: Find latest component matching semver 
    Component Version Controller->>OCM Repository: Validate signatures
    Component Version Controller->>OCM Repository: Download Component Descriptor
    Component Version Controller->>Kubernetes API: Submit Component Descriptor CR
    Component Version Controller->>Kubernetes API: Update Component Version status
```

The custom resource for the component version controller looks as follows:

```yaml
apiVersion: delivery.ocm.software/v1alpha1
kind: ComponentVersion
metadata:
  name: component-x
  namespace: default
spec:
  interval: 10m0s
  component: github.com/open-component-model/component-x
  version:
    semver: ">=v1.0.0"
  repository:
    url: ghcr.io/jane-doe
    secretRef:
      name: ghcr-creds
  verify:
    - name: dev-signature
      publicKey:
        secretRef:
          name: signing-key
```

#### Resource Controller

The resource controller extracts resources from a component so that they may be used within the cluster. The resource is written to a snapshot which enables it to be cached and used by downstream processes. Resources can be selected using the `name` and `extraIdentity` fields. The resource controller requests resources using the in-cluster registry client. This means that if a resource has previously been requested then the cached version will be returned. If the resource is not found in the cache then it will be fetched from the OCM registry and written to the cache. Once the resource has been resolved and is stored in the internal registry a Snapshot CR is created 

```mermaid
sequenceDiagram
    User->>Kubernetes API: submit Resource CR
    Kubernetes API-->>Resource Controller: Resource Created Event
    Resource Controller->>Internal Registry: Fetch resource from cache or upstream
    Resource Controller->>Kubernetes API: Create Snapshot CR
    Resource Controller->>Kubernetes API: Update Resource status
```

The custom resource for the Resource controller is as follows:

```yaml
apiVersion: delivery.ocm.software/v1alpha1
kind: Resource
metadata:
  name: manifests
spec:
  interval: 10m0s
  sourceRef:
    kind: ComponentVersion
    name: component-x
    namespace: default
    resourceRef:
      name: manifests
      referencePath:
        - name: nested-component
```

#### Snapshot Controller

The Snapshot controller reconciles Snapshot Custom Resources. Currently, the functionality here is limited to updating the status thereby validating that the snapshotted resource exists. In the future we plan to expand the scope of this controller to include verification of snapshots.

#### Localization Controller

The localization controller applies localization rules to a snapshot. Because localization is deemed a common operation it is included along with the configuration controller in the ocm-controller itself. Localizations can consume an OCM resource directly or a snapshot resource from the in-cluster registry. The configuration details for the localization operation are supplied via another OCM resource which should be a yaml file in the following format:

```yaml
apiVersion: config.ocm.software/v1alpha1
kind: ConfigData
metadata:
  name: ocm-config
  labels:
    env: test
localization:
- resource:
    name: image
  file: deploy.yaml
  image: spec.template.spec.containers[0].image
```

Localization parameters are specified under the `localization` stanza. The Localization controller will apply the localization rules that apply to the resource specified in the `sourceRef` field. 

```mermaid
sequenceDiagram
    User->>Kubernetes API: submit Localization CR
    Kubernetes API-->>Localization Controller: Localization Created Event
    Localization Controller->>Internal Registry: Fetch resource from cache or upstream
    Localization Controller->>Internal Registry: Fetch configuration resource from cache or upstream
    Localization Controller->>Localization Controller: Apply matching localization rules
    Localization Controller->>Internal Registry: Push localized resource to internal registry
    Localization Controller->>Kubernetes API: Create Snapshot CR
    Localization Controller->>Kubernetes API: Update Localization status
```

The custom resource for the Localization controllers is as follows:

```yaml
apiVersion: delivery.ocm.software/v1alpha1
kind: Localization
metadata:
  name: manifests
spec:
  interval: 1m
  sourceRef:
    kind: Resource
    name: manifests
    resourceRef:
      name: image
      version: latest
  configRef:
    kind: ComponentVersion
    name: component-x
    resourceRef:
      name: config
      version: latest
```

#### Configuration Controller

The configuration controller is used to configure resources for a particular environment and similar to localization the configured resource is written to a snapshot. Because configuration is deemed a common operation it is included along with the configuration controller in the ocm-controller itself. The behaviour is as described for the localization controller but instead of retrieving configuration from the `localization` stanza of the `ConfigData` file, the controller retrieves configuration information from the `configuration` stanza:

```yaml
apiVersion: config.ocm.software/v1alpha1
kind: ConfigData
metadata:
  name: ocm-config
  labels:
    env: test
configuration:
  defaults:
    color: red
    message: Hello, world!
  schema:
    type: object
    additionalProperties: false
    properties:
      color:
        type: string
      message:
        type: string
  rules:
  - value: (( message ))
    file: configmap.yaml
    path: data.PODINFO_UI_MESSAGE
  - value: (( color ))
    file: configmap.yaml
    path: data.PODINFO_UI_COLOR
```

And a configuration object might something like this:

```yaml
apiVersion: delivery.ocm.software/v1alpha1
kind: Configuration
metadata:
  name: configuration
spec:
  interval: 1m0s
  sourceRef: # we configure the localized data
    kind: Localization
    name: manifests
  configRef:
    kind: ComponentVersion
    name: component-x
    resourceRef:
      name: config
      version: latest
  valuesFrom:
    fluxSource:
      sourceRef:
        kind: GitRepository # get the values from a git repository provided by flux
        name: flux-system
        namespace: flux-system
      path: ./values.yaml
      subPath: component-x-configs
```

### FluxDeployer controller

The final piece in this puzzle is the deployment object. _Note_ this might change in the future to provide more deployment
options.

Current, Flux is implemented using the `FluxDeployer` API. This provides a connection with Flux's ability to apply
manifest files taken from an OCI repository. Here, the OCI repository is the in-cluster registry and the path to it
will be provided by the snapshot created by the last link in the chain.

Consider the following example using the localized and configured resource from above:

```yaml
apiVersion: delivery.ocm.software/v1alpha1
kind: FluxDeployer
metadata:
  name: fluxdeployer-podinfo-pipeline-frontend
spec:
  interval: 1m0s
  sourceRef:
    kind: Configuration
    name: configuration
  kustomizationTemplate:
    interval: 5s
    path: ./
    prune: true
    targetNamespace: ocm-system
```

This will deploy any manifest files at path `./` in the result of the above configuration.

### Replication controller

The Replication Controller handles the replication of components between OCI repositories. It consists of a single reconciler which manages subscriptions to a source OCI repository. A semver constraint is used to specify a target component version. Component versions satisfying the semver constraint will be copied to the destination OCI repository. The replication controller will verify signatures before performing replication.

```yaml
apiVersion: delivery.ocm.software/v1alpha1
kind: ComponentSubscription
metadata:
  name: componentsubscription-sample
  namespace: ocm-system
spec:
  source:
    secretRef:
      name: source-access-secret
    url: oci://source
  destination:
    secretRef:
      name: destination-access-secret
    url: oci://destination
  component: "https://github.com/open-component-model/component-x"
  interval: 10m0s
  semver: "~v0.1.0"
  verify:
    - signature:
        name: signature-name
        key:
          name: verify-key-name
status:
  latestVersion: "v0.1.1"
  replicatedVersion: "v0.1.0"
```

```mermaid
sequenceDiagram
    User->>Kubernetes API: submit Component Subscription CR
    Kubernetes API-->>Replication Controller: Component Subscription Created Event
    Replication Controller->>Replication Controller: Determine new component is available in source repository based on semver
    Replication Controller->>Source OCM Repository: Verify signatures 
    Source OCM Repository->>Destination OCM Repository: Transfer component by value
    Replication Controller->>Kubernetes API: Update Component Subscription status
```

## In-cluster Docker Registry

The `ocm-controller` manages a deployment of the docker registry. This provides a caching mechanism for resources and storage for snapshots whilst also enabling integration with Flux. Usage of the in-cluster registry is transparent to the clients and is handled via the ocm client library provided by the controller sdk.
