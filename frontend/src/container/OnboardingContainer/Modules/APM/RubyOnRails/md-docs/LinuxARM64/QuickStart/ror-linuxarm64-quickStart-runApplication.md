### Running your Ruby application

Run the application using the below:

```bash
OTEL_EXPORTER=otlp \
OTEL_SERVICE_NAME={{MYAPP}} \
OTEL_EXPORTER_OTLP_ENDPOINT=https://ingest.{{REGION}}.o11y.hanzo.ai:443 \
OTEL_EXPORTER_OTLP_HEADERS=signoz-ingestion-key={{HANZO_INGESTION_KEY}} \
rails server
```