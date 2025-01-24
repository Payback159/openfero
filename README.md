# OpenFero

Open Fero is a little play on words from the Latin "opem fero", which means "to help" and the term "OpenSource". Hence the name "openfero". The scope of OpenFero is a framework for self-healing in a cloud-native environment.

[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/OpenFero/openfero/badge)](https://scorecard.dev/viewer/?uri=github.com/OpenFero/openfero) [![OpenSSF Best Practices](https://www.bestpractices.dev/projects/6683/badge)](https://www.bestpractices.dev/projects/6683)

## Getting started

The recommended method is installation via a helm chart.

```bash
helm pull oci://ghcr.io/openfero/openfero/charts/openfero --version 0.1.0
helm install openfero oci://ghcr.io/openfero/openfero/charts/openfero --version 0.1.0
```

## Component-Diagram

![Shows the Prometheus, Alertmanager components and that Alertmanager notifies the OpenFero component so that OpenFero starts the jobs via Kubernetes API.][comp-dia]

## Operarios definitions

The operarios definitions are stored in the namespace in ConfigMaps with the naming convention `openfero-<alertname>-<status>`.

### Example-Names

- `openfero-KubeQuotaAlmostReached-firing`
- `openfero-KubeQuotaAlmostReached-resolved`

### Operarios-Example

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: openfero-kubequotaalmostfull-firing
  labels:
    app: openfero
spec:
  parallelism: 1
  completions: 1
  template:
    labels:
      app: openfero
    spec:
      containers:
        - name: python-job
          image: python:latest
          args:
            - bash
            - -c
            - |-
              echo "Hallo Welt"
      imagePullPolicy: Always
      restartPolicy: Never
      serviceAccount: <desired-sa>
      serviceAccountName: <desired-sa>
```

## Security note

The service account that is installed when deploying openfero is for openfero itself. For the operarios, separate service accounts must be rolled out, which have the appropriate permissions for the remediation.

For operarios that need to interact with the Kubernetes API, it is recommended to define a suitable role for and authorize it via ServiceAccount in the job definition.

[comp-dia]: ./docs/component-diagram.png
