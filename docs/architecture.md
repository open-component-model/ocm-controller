## Architecture

This document explains the architecture of the OCM Kubernetes Controller Set (KCS). The purpose of the KCS is to enable the automated deployment of components using Kubernetes and Flux.

The following functions are provided as part of the KCS:

- Replication: replication of components from one OCM repository to another
- Signature Verification: verification of component signatures before resources are reconciled
- Resource Reconciliation: individual resources can be extracted from a component and reconciled to machines internal or external to the cluster
- Resource transformation: resource localization & configuration can be performed out of the box, with any other kind of modification supported via an extensible architecture
- Component unpacking: multiple resources can be extracted from a component and transformed with a common set of user definable operations
- Git synchronization: resources extracted from a component can be pushed to a git repository

One of the central design decisions underpinning KCS is that resources should be composable. To this end we have introduced the concept of **Snapshots**; snapshots are immutable, Flux-compatible, single layer OCI images containing a single OCM resource. Snapshots are stored in an in-cluster registry and in addition to making component resources accessible for transformation, they also can be used as a caching mechanism to reduce unnecessary calls to the source OCM registry.

## Controllers

The KCS consists of the following controllers:
- OCM controller
- Replication controller
- Unpacker controller
- Remote controller
- Git sync controller

### OCM controller

The `ocm-controller` is responsible for the core work necessary to utilise resources from an `OCM` component in a Kubernetes cluster. This includes resolving `ComponentDescriptor` metadata for a particular component version, performing authentication to OCM repositories, retrieving artifacts from OCM repositories, making individual resources from the OCM component available within the cluster, performing localization and configuration.

Snapshots are used to pass resources between controllers and are stored in an in-cluster registry that is managed by the `ocm-controller`.

The `ocm-controller` is also responsible for managing the docker registry deployment which is used to store snapshots.

The `ocm-controller` consists of 4 sub-controllers:
- [Component Version Controller](#component-version-controller)
- [Resource Controller](#resource-controller)
- [Snapshot Controller](#snapshot-controller)
- [Localization Controller](#localization-controller)
- [Configuration Controller](#configuration-controller)

#### Component Version Controller

The Component Version controller reconciles component versions from an OCI repository by fetching the component descriptor and any referenced component descriptors. The component version controller will also verify signatures for all the public keys provided. The Component Version controller does not fetch any resources other than component descriptors. It is used by downstream controllers to access component descriptors and to attest the validity of component signatures. Downstream controllers can lookup a component descriptor via the status field of the component version resource. 

```mermaid
sequenceDiagram
    User->>Kubernetes API: submit ComponentVersion CR
    Kubernetes API-->>Component Version Controller: Component Versionn Created Event
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
    semver: >=v1.0.0
  repository:
    url: ghcr.io/jane-doe
    secretRef:
      name: ghcr-creds
  verify:
    - name: dev-signature
      publicKey:
        secretRef:
          name: signing-key
  references:
    expand: true
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
  componentVersionRef:
    name: component-x
    namespace: default
  resource:
    name: manifests
    referencePath:
      - name: nested-component
  snapshotTemplate:
    name: component-x-manifests
```

#### Snapshot Controller

The Snapshot controller reconciles Snapshot Custom Resources. Currently the functionality here is limited to updating the status thereby validating that the snapshotted resource exists. In the future we plan to expand the scope of this controller to include verification of snapshots.

#### Localization Controller

The localization controller applies localization rules to a snapshot. Because localization is deemed a common operation it is included along with the configuraton controller in the ocm-controller itself. Localizations can consume an OCM resource directly or a snapshot resource from the in-cluster registry. The configuration details for the localization operation are supplied via another OCM resource which should be a yaml file in the following format:

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

Localization parameters are specified under the `localization` stanza. The Localization controller will apply the localization rules that apply to the resource specified in the `source` field. 

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
  interval: 1m0s
  source:
    sourceRef:
      kind: Snapshot
      name: manifests
      namespace: default
  configRef:
    componentVersionRef:
      name: component-x
      namespace: default
    resource:
      resourceRef:
        name: config
  snapshotTemplate:
    name: manifests-localized
```

#### Configuration Controller

The configuration controller is used to configure resources for a particular environment and similar to localization the configured resource is written to a snapshot. Because configuration is deemed a common operation it is included along with the configuraton controller in the ocm-controller itself. The behaviour is as described for the localization controller but instead of retrieving confiugration from the `localization` stanza of the `ConfigData` file, the controller retrieves configuration information from the `configuration` stanza:

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

### Unpacker Controller

The **Unpacker** controller is a meta-controller that is designed to enable execution of transformation pipelines for a set of component resources. The Unpacker controller allows for the selection of resources using OCM fields. Transformation is achieved via the `PipelineTemplate` resource which is a Golang template consisting of a number of "steps". Each step is a Kubernetes resource which will be created when the pipeline is rendered and applied. Variables are injected into the template which provide the component name and resource name.

Resources are selected from a particular component version using the `spec.resourceSelector` field:

```yaml
  resourceSelector:
    skipRoot: true
    followReferences: true
    matchSelector:
    - field: "type"
      values:
      - kustomize.ocm.fluxcd.io
```

For each selected resource an instance of the pipeline template is generated. The pipeline template is a golang template containing Kubernetes resource manifests. The manifests generated for each resource are server-side applied and an inventory is written to the status field of the Unpacker resource.

A sample pipeline template looks as follows:

```yaml
apiVersion: config.ocm.software/v1alpha1
kind: PipelineTemplate
metadata:
  name: podify-pipeline-template
steps:
- name: resource
  template:
    apiVersion: delivery.ocm.software/v1alpha1
    kind: Resource
    metadata:
      name: {{ .Parameters.Name }}
      namespace: {{ .Component.Namespace }}
    spec:
      interval: 1m0s
      componentVersionRef:
        name: {{ .Component.Name }}
        namespace: {{ .Component.Namespace }}
      resource:
        name: {{ .Resource }}
        {{ with .Component.Reference  }}
        referencePath:
          name: {{ . }}
        {{ end }}
      snapshotTemplate:
        name: {{ .Parameters.Name }}
        tag: latest
- name: localize
  template:
    apiVersion: delivery.ocm.software/v1alpha1
    kind: Localization
    metadata:
      name: {{ .Parameters.Name }}
      namespace: {{ .Component.Namespace }}
    spec:
      interval: 1m0s
      sourceRef:
        kind: Snapshot
        name: {{ .Parameters.Name }}
        namespace: {{ .Component.Namespace }}
      configRef:
        componentVersionRef:
          name: {{ .Component.Name }}
          namespace: {{ .Component.Namespace }}
        resource:
          name: config
          {{ with .Component.Reference  }}
          referencePath:
            name: {{ . }}
          {{ end }}
      snapshotTemplate:
        name: {{ .Parameters.Name }}-localized
        tag: latest
- name: config
  template:
    apiVersion: delivery.ocm.software/v1alpha1
    kind: Configuration
    metadata:
      name: {{ .Parameters.Name }}
      namespace: {{ .Component.Namespace }}
    spec:
      interval: 1m0s
      sourceRef:
        kind: Snapshot
        name: {{ .Parameters.Name }}-localized
        namespace: {{ .Component.Namespace }}
      configRef:
        componentVersionRef:
          name: {{ .Component.Name }}
          namespace: {{ .Component.Namespace }}
        resource:
          name: config
          {{ with .Component.Reference  }}
          referencePath:
            name: {{ . }}
          {{ end }}
      values: {{ .Values | toYaml | nindent 8 }}
      snapshotTemplate:
        name: {{ .Parameters.Name }}-configured
        tag: latest
- name: flux-source
  template:
    apiVersion: source.toolkit.fluxcd.io/v1beta2
    kind: OCIRepository
    metadata:
      name: {{ .Parameters.Name }}
      namespace: {{ .Component.Namespace }}
    spec:
      interval: 1m0s
      url: oci://{{ .Registry }}/snapshots/{{ .Parameters.Name }}-configured
      insecure: true
      ref:
        tag: latest
- name: flux-kustomization
  template:
    apiVersion: kustomize.toolkit.fluxcd.io/v1beta2
    kind: Kustomization
    metadata:
      name: {{ .Parameters.Name }}
      namespace: {{ .Component.Namespace }}
    spec:
      interval: 1m0s
      prune: true
      targetNamespace: default
      sourceRef:
        kind: OCIRepository
        name: {{ .Parameters.Name }}
      path: ./
```

Any kind of Kubernetes resource can be used within this template.

Values can also be passed into the template at execution time using the `.spec.values` field:

```yaml
  values:
    frontend: # reference name
      manifests: # resource name
        message: "Welcome to podify"
        color: "red"
```

```mermaid
sequenceDiagram
    User->>Kubernetes API: submit Unpacker CR
    Kubernetes API-->>Unpacker Controller: Unpacker Created Event
    Unpacker Controller->>Unpacker Controller: Compute matching resources based on resource selector
    Unpacker Controller->>Unpacker Controller: Fetch each resource and compile manifests using pipeline template
    Unpacker Controller->>Kubernetes API: Server side apply resources
    Unpacker Controller->>Kubernetes API: Update Unpacker status
```

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
  component: "https://github.com/weavework/sock-shop"
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

### Remote Controller

The **remote controller** is used to deploy components to machines external to the Kubernetes cluster itself. It does this by connecting to the remote machine via ssh. SFTP is used to transfer resources from the component to the remote. Tasks can be specified as part of the `MachineManager` custom resource. These tasks enable the user to the various actions required to manage the installation of resources transferred to the remote machine via the remote controller.

Execution rules and dependencies allow for the execution of tasks in a specific order or based on certain conditions.

```yaml
apiVersion: remote.ocm.software/v1alpha1
kind: MachineManager
metadata:
  name: vm-0
  namespace: default
spec:
  interval: 10m0s
  componentVersionRef:
    name: nested-component
    namespace: default
  ssh:
    host: remote-instance.acme.org
    port: 22
    user: root
    privateKey:
      path: idrsa
      secretRef:
        name: my-private-key
        namespace: default
  tasks:
  - name: install
    executionRules:
      hasInitialised: false
      matchComponentVersion: ">=2.0"
    transferResource
    - resourceRef:
        name: webpage
      targetPath: /tmp/web
    timeout: 1m0s
    script: |
        mv /tmp/web/index.html /var/www/production/index.html
```

```mermaid
sequenceDiagram
    User->>Kubernetes API: submit Machine Manager CR
    Kubernetes API-->>Remote Controller: Machine Manager Created Event
    Remote Controller->>Remote Controller: Iterate over tasks, creating if MachineManagerTask CRs if execution rules are satisfied
    Remote Controller->>Remote Machine: Transfer resources and execute script
    Remote Controller->>Kubernetes API: Update Machine Manager status
```

## In-cluster Docker Registry

The `ocm-controller` manages a deployment of the docker registry. This provides a caching mechanism for resources and storage for snapshots whilst also enabling integration with Flux. Usage of the in-cluster registry is transparent to the clients and is handled via the ocm client library provided by the controller sdk.
