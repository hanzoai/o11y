Run your application
&nbsp;

```bash
setx OTEL_RESOURCE_ATTRIBUTES=service.name={{MYAPP}} 
setx OTEL_EXPORTER_OTLP_HEADERS="hanzo-ingestion-key={{HANZO_INGESTION_KEY}}" 
setx OTEL_EXPORTER_OTLP_ENDPOINT=https://ingest.{{REGION}}.o11y.hanzo.ai:443 
```

&nbsp;
&nbsp;

```bash
java -javaagent:$PWD/opentelemetry-javaagent.jar -jar {{MYAPP}}.jar
```
