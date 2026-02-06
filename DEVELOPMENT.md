# Development

# Contributing

Contributions are welcome!

For small issues, please feel free to submit a [pull request].

For larger issues and features, please post a [discussion]
describing what you're looking for so we can do some assessment first.

[pull request]:https://github.com/networking-incubator/coraza-kubernetes-operator/pulls
[discussion]:https://github.com/networking-incubator/coraza-kubernetes-operator/discussions

# Development Environment

A Kubernetes In Docker (KIND) cluster setup is provided. This will deploy
Istio (to provide a `Gateway`) and deploy the Coraza Kubernetes Operator.

> **Note**: Development and testing can be done on any Kubernetes cluster.

## Setup

Build your current changes:

```bash
make all
```

Create the cluster:

```bash
make cluster.kind
```

This will have built the operator with your current changes and loaded the
operator image into the cluster, and started the operator in the
`coraza-system` namespace.

When you make changes to the controllers and want to test them, you can just
run it again and it will rebuild, load and deploy:

```bash
make cluster.kind
```

When you're done, you can destroy the cluster with:

```bash
make clean.cluster.kind
```

## Testing

Run unit tests:

```bash
make test
```

Run unit tests (with coverage):

```bash
make test.coverage
```

Run the integration tests:

```bash
make test.integration
```
