# Contributions

Welcome to the OCM community!

We welcome many types of contributions.

Please refer to the [Contributing Guide in the Community repository](https://github.com/open-component-model/community/blob/main/CONTRIBUTING.md) for more information on how to get support from maintainers, find work to contribute, the Pull Request checklist, the Pull Request process, and other useful information on how to contribute to OCM.

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

If in-cluster is the preferred way to test the controller, create a kind registry that kind can pull images from.
To do that follow [this](https://kind.sigs.k8s.io/docs/user/local-registry/) guide.

Once a local registry exists, build the controller image with:

```
docker build -t localhost:5001/ocm-controller/manager:v0.0.1 .
```

When it's done, push it to the local registry:

```
docker push localhost:5001/ocm-controller/manager:v0.0.1
```

Now the controller can be installed onto the cluster. Before that, install all crds and rbacs with `make install`.
There is a deployment manifest located [here](config/manager/deployment.yaml) which can be used to install the
controller.

_Note_: If a local kind cluster with local registry is used, change the `image: ` value from 
`image: open-component-model/ocm-controller` to `localhost:5001/ocm-controller/manager:v0.0.1` so the Kind workers can
pull to image from the local registry.

If it's enough to run the manager locally, you don't need a local registry. Simply run:

```
./bin/manager -zap-log-level 4
```
