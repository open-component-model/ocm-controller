#!/bin/bash

check_dependency() {
  # Check if helm is installed
  if ! command -v helm &> /dev/null
  then
      echo "Helm could not be found. Please install Helm to proceed."
      exit
  fi
  # Check if kustomize is installed
    if ! command -v kustomize &> /dev/null
    then
        echo "kustomize could not be found. Please install kustomize to proceed."
        exit
    fi
}

create_helm_chart() {
  helm create "${1}/ocm-controller"
  cd "${1}/ocm-controller"
  rm -r templates
  mkdir templates
  mkdir crds
  rm -r charts
  rm values.yaml
  cd ../..

  mkdir -p "${1}/helm_temp"
  split -p "^---$" "${1}/install.yaml" "${1}/helm_temp/helm_";
  HELM_FILES=($(ls -1 "${1}/helm_temp" ))

  # Set chart app versiom to release version
  RELEASE=$(ls -t docs/release_notes | head -1 | sed "s/v//g" | sed "s/.md//g")
  sed -i "" "s/appVersion: .*/appVersion: \"${RELEASE}\"/g" "${1}/ocm-controller/Chart.yaml"

  #Move into crds & templates folders after renaming
  for input in "${HELM_FILES[@]}";  do
      FILENAME=$(cat "${1}/helm_temp/${input}" | grep '^  name: ' | head -1 | sed 's/name: //g' | sed 's/\./_/g' | sed 's/ //g')
      TYPE=$(cat "${1}/helm_temp/${input}" | grep '^kind: ' | head -1 | sed 's/kind: //g' | sed 's/\./_/g' | tr '[:upper:]' '[:lower:]' | sed 's/ //g')
      if grep -q "kind: CustomResourceDefinition" "${1}/helm_temp/${input}" ; then
          mv ${1}/helm_temp/${input} ${1}/ocm-controller/crds/${FILENAME}.yaml
      else
          mv ${1}/helm_temp/${input} ${1}/ocm-controller/templates/${TYPE}_${FILENAME}.yaml
      fi
  done

  rm -rf ${1}/helm_temp
}

create_from_local_resource_manifests() {
  check_dependency
  echo "Creating from local manifests in the repository"
  rm -rf helm
  mkdir helm
  kustomize build ./config/default > ./helm/install.yaml

  create_helm_chart "helm"
  rm helm/install.yaml
}


create_from_github_release() {
  echo "Creating from the latest release available on github.com/open-component-model"
  create_helm_chart "output"
}

case "${1}" in
    "local" )
          create_from_local_resource_manifests
          ;;
    "release" )
          create_from_github_release
          ;;
    * )
          echo -n "unknown"
          exit
          ;;
esac
