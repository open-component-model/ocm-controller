## Resource Pipeline Example

### Build the component

```shell
ocm add component componentfile.yaml --create --file .ocm --scheme=v3alpha1
```

### Transfer the component

```shell
ocm transfer component -f .ocm ghcr.io/$GITHUB_USER
```

### Deploy the Component Version

```shell
kubectl apply -f cv.yaml
```

### Create the Resource Pipeline

```shell
kubectl apply -f pipeline.yaml
```

### Create the Flux Deployer

```shell
kubectl apply -f deployer.yaml
```

### View the results

```shell
kubectl get pods -n default
```
