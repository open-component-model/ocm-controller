#!/usr/bin/env bash

# setup flux cluster reconilication

flux create source git flux-system \
  --url=https://github.com/open-component-model/ocm-controller \
  --branch=${BRANCH} \
  --username=${GITHUB_USER} \
  --password=${GITHUB_TOKEN} \
  --ignore-paths="clusters/**/flux-system/"

flux create kustomization flux-system \
  --source=flux-system \
  --path=./deploy/flux/infra
