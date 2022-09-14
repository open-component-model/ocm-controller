# ocm-controller
Main repository for ocm-controller.

## Testing

In order to test, run the manager locally with a kind cluster present on the system.

- apply all CRDs from the crd/base folder
- run the controller:
```console
make
./bin/manager -zap-log-level 4
```
- apply all the sample objects
