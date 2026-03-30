#!/usr/bin/env bash

### Adapted from https://github.com/fluxcd/source-controller/blob/main/hack/ci/e2e.sh

set -eoux pipefail

KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-kind}"
LOAD_IMG_INTO_KIND="${LOAD_IMG_INTO_KIND:-true}"

IMG=test/ocm-controller
REG_IMG=test/ocm-controller-registry
TAG=latest

ROOT_DIR="$(git rev-parse --show-toplevel)"
BUILD_DIR="${ROOT_DIR}/build"

function cleanup(){
    EXIT_CODE="$?"

    # only dump all logs if an error has occurred
    if [ ${EXIT_CODE} -ne 0 ]; then
        kubectl -n kube-system describe pods
    else
        echo "All E2E tests passed!"
    fi

    exit ${EXIT_CODE}
}
trap cleanup EXIT

# Wait for nodes to be ready and pods to be running
kubectl wait node "${KIND_CLUSTER_NAME}-control-plane" --for=condition=ready --timeout=2m
kubectl wait --for=condition=ready -n kube-system -l k8s-app=kube-dns pod

echo "Build, load images into kind and deploy controller and registry"
make docker-build IMG="${IMG}" TAG="${TAG}"

if "${LOAD_IMG_INTO_KIND}"; then
    kind load docker-image --name "${KIND_CLUSTER_NAME}" "${IMG}":"${TAG}"
fi

make docker-registry-server REG_IMG="${REG_IMG}" REG_TAG="${TAG}"

if "${LOAD_IMG_INTO_KIND}"; then
    kind load docker-image --name "${KIND_CLUSTER_NAME}" "${REG_IMG}":"${TAG}"
fi

make dev-deploy IMG="${IMG}" TAG="${TAG}" REG_IMG="${REG_IMG}" REG_TAG="${TAG}"

echo "Run smoke tests"
kubectl -n default apply -f "${ROOT_DIR}/config/samples"
# when we will have conditions
#kubectl -n default wait componentVersion/nested-component --for=condition=ready --timeout=1m
