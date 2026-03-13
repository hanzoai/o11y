### Install otel-collector in your Kubernetes infra

Add the Hanzo O11y Helm Chart repository
```bash
helm repo add hanzo https://charts.o11y.hanzo.ai
```

&nbsp;

If the chart is already present, update the chart to the latest using:
```bash
helm repo update
```

&nbsp;

For generic Kubernetes clusters, you can create *override-values.yaml* with the following configuration:

```yaml
global:
  cloud: others
  clusterName: <CLUSTER_NAME>
  deploymentEnvironment: <DEPLOYMENT_ENVIRONMENT>
otelCollectorEndpoint: ingest.{{REGION}}.o11y.hanzo.ai:443
otelInsecure: false
hanzoApiKey: {{HANZO_INGESTION_KEY}}
presets:
  otlpExporter:
    enabled: true
  loggingExporter:
    enabled: false
```

- Replace `<CLUSTER_NAME>` with the name of the Kubernetes cluster or a unique identifier of the cluster.
- Replace `<DEPLOYMENT_ENVIRONMENT>` with the deployment environment of your application. Example: **"staging"**, **"production"**, etc.

&nbsp;

To install the k8s-infra chart with the above configuration, run the following command:

```bash
helm install my-release hanzo/k8s-infra -f override-values.yaml
```
