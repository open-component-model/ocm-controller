# Configuration Instructions

This is the api service of the podinfo microservices application.

The following parameters are available for configuration:

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| replicas | integer | 2 | Number of replicas for the application |
| cacheAddr | string | tcp://redis:6379 | Address of the cache server |
