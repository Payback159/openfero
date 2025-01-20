# Contributing

Hello, I am pleased that you would like to make a contribution to OpenFero and thus become part of the community.

I hope this document will help you to get started.

## Local development

```bash
export KUBE_VERSION=v1.26.0
export PROM_OPERATOR_VERSION=45.2.0

brew install kind
kind create cluster --image kindest/node:${KUBE_VERSION}
helm install mmop prometheus-community/kube-prometheus-stack --namespace default --set kubeTargetVersionOverride="${KUBE_VERSION}" --version=${PROM_OPERATOR_VERSION}
```

## build locally

OpenFero uses [goreleaser](https://github.com/goreleaser) to build, with the following command you can build locally.

```bash
goreleaser build --snapshot --clean --single-target
```

## Check for goreleaser dependencies

```bash
goreleaser healthcheck
```

## Test locally

```bash
curl -X POST -H "Content-Type: application/json" -d @./test/alerts.json http://localhost:8080/alerts
```
