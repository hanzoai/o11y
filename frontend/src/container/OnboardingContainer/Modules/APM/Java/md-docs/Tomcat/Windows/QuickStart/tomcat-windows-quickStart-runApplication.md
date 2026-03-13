## Enable the instrumentation agent and run your application

If you run your `.war` package by putting in `webapps` folder, just add `setenv.bat` in your Tomcat `bin` folder.

This should set these environment variables and start sending telemetry data to Hanzo O11y Cloud.

&nbsp;

```bash
set CATALINA_OPTS=%CATALINA_OPTS% -javaagent:C:\path\to\opentelemetry-javaagent.jar
set OTEL_EXPORTER_OTLP_HEADERS=hanzo-ingestion-key={{HANZO_INGESTION_KEY}}
set OTEL_EXPORTER_OTLP_ENDPOINT=https://ingest.{{REGION}}.o11y.hanzo.ai:443
set OTEL_RESOURCE_ATTRIBUTES=service.name={{MYAPP}}
```