## Architecture

This document explains the architecture of the OCM Kubernetes Controller Set (ORCA). The purpose of the KCS is to enable the automated deployment of components using Kubernetes and Flux.

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

The `ocm-controller` is responsible for creating the docker registry deployment which is used to store snapshots.

The `ocm-controller` consists of 4 sub-controllers:

#### ComponentVersion Controller

The ComponentVersion controller reconciles component versions from an OCI repository by fetching the component descriptor and any referenced component descriptors. The component version controller will also verify signatures for all the public keys provided.

#### Resource Controller

The resource controller extracts resources from a component so that they may be used within the cluster. The resource is written to a snapshot which enables it to be cached and used by downstream processes. Resources can be selected using the `name` and `extraIdentity` fields.

#### Snapshot Controller

The Snapshot controller reconciles Snapshot Custom Resources. Currently the functionality here is limited to updating the status. In the future we plan to expand the scope of this controller to include verification of snapshots.

#### Localization Controller

The localization controller applies localization rules to a snapshot. Because localization is such a common operation it is included along with the configuraton controller in the ocm-controller itself. The localized resource is written to a snapshot.

#### Configuration Controller

The configuration controller is used to configure a snapshot, similar to localization the configured resources is written to a snapshot.

### Unpacker Controller

The **Unpacker** controller is a meta-controller that is designed to enable execution of transformation pipelines for a set of component resources. The Unpacker controller allows for the selection of resources using OCM fields. Transformation is achieved via the PipelineTemplate resource which is a Golang template consisting of a number of "steps". Each step is a Kubernetes resource which will be created when the pipeline is rendered and applied. Variables are injected into the template which provide the component name and resource name.

### Replication controller

The Replication Controller handles the replication of components between OCI repositories. It consists of a single reconciler which manages subscriptions to a source OCI repository. A semver constraint is used to specify a target component version. Component versions satisfying the semver constraint will be copied to the destination OCI repository. The replication controller will verify signatures before performing replication.

### Remote Controller

The **remote controller** is used to deploy components to machines external to the Kubernetes cluster itself. It does this by connecting to the remote machine via ssh. SFTP is used to transfer resources from the component to the remote. Scripts can be specified as part of the `MachineManager` custom resource. These scripts enable the user to the various actions required to manage the installation of resources transferred to the remote machine via the remote controller.

## In-cluster Docker Registry

The `ocm-controller` manages a deployment of the docker registry. This provides a caching mechanism for resources and storage for snapshots whilst also enabling integration with Flux. Usage of the in-cluster registry is transparent to the clients and is handled via the ocm client library provided by the controller sdk.
