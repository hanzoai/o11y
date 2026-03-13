## Requirements

Java 8 or higher

## Send Traces to Hanzo O11y Cloud

OpenTelemetry provides a handy Java JAR agent that can be attached to any Java 8+ application and dynamically injects bytecode to capture telemetry from a number of popular libraries and frameworks.

Based on your application environment, you can choose the setup below to send traces to Hanzo O11y Cloud.

### Application on VMs

From VMs, there are two ways to send data to Hanzo O11y Cloud.

- Send traces directly to Hanzo O11y Cloud (quick start)
- Send traces via OTel Collector binary (recommended)

#### **Send traces directly to Hanzo O11y Cloud**
OpenTelemetry Java agent can send traces directly to Hanzo O11y Cloud.
  
Step 1. Download otel java binary agent

```bash
wget https://github.com/open-telemetry/opentelemetry-java-instrumentation/releases/latest/download/opentelemetry-javaagent.jar
```

Step 2. Enable the instrumentation agent and run your application

If you run your `.war` package by putting in `webapps` folder, just add `setenv.sh` in your Tomcat `bin` folder. Inside the `setenv.sh` file, add the following environment variables: 


```bash
export CATALINA_OPTS="$CATALINA_OPTS -javaagent:/<path>/opentelemetry-javaagent.jar"
export OTEL_EXPORTER_OTLP_HEADERS="hanzo-ingestion-key={{HANZO_INGESTION_KEY}}"
export OTEL_EXPORTER_OTLP_ENDPOINT=https://ingest.{{REGION}}.o11y.hanzo.ai:443
export OTEL_RESOURCE_ATTRIBUTES=service.name={{MYAPP}}
```

- `<path>` - update it to the path of your downloaded Java JAR agent.

---

#### **Send traces via OTel Collector binary**

OTel Collector binary helps to collect logs, hostmetrics, resource and infra attributes. It is recommended to install Otel Collector binary to collect and send traces to Hanzo O11y cloud. You can correlate signals and have rich contextual data through this way.

You can find instructions to install OTel Collector binary [here](https://o11y.hanzo.ai/docs/tutorial/opentelemetry-binary-usage-in-virtual-machine/) in your VM. Once you are done setting up your OTel Collector binary, you can follow the below steps for instrumenting your Java application.

Step 1. Download OTel java binary agent
```bash
wget https://github.com/open-telemetry/opentelemetry-java-instrumentation/releases/latest/download/opentelemetry-javaagent.jar
```

Step 2. Enable the instrumentation agent and run your application

If you run your `.war` package by putting in `webapps` folder, just add `setenv.sh` in your Tomcat `bin` folder.

This should set these environment variables and start sending telemetry data to Hanzo O11y Cloud.


```bash
export CATALINA_OPTS="$CATALINA_OPTS -javaagent:/<path>/opentelemetry-javaagent.jar"
```

- `<path>` - Update it to the path of your downloaded Java JAR agent.

---
  
### Applications Deployed on Kubernetes

For Java application deployed on Kubernetes, you need to install OTel Collector agent in your k8s infra to collect and send traces to Hanzo O11y Cloud. You can find the instructions to install OTel Collector agent [here](https://o11y.hanzo.ai/docs/tutorial/kubernetes-infra-metrics/).

Once you have set up OTel Collector agent, you can proceed with OpenTelemetry java instrumentation by following the below steps:

Step 1. Download otel java binary

```bash
wget https://github.com/open-telemetry/opentelemetry-java-instrumentation/releases/latest/download/opentelemetry-javaagent.jar
```

Step 2. Enable the instrumentation agent and run your application

If you run your `.war` package by putting in `webapps` folder, just add `setenv.sh` in your Tomcat `bin` folder.

This should set the environment variable and start sending telemetry data to Hanzo O11y Cloud.

```bash
export CATALINA_OPTS="$CATALINA_OPTS -javaagent:/<path>/opentelemetry-javaagent.jar"
```

- `<path>` - Update it to the path of your downloaded Java JAR agent.

Step 3. Make sure to dockerise your application along with OpenTelemetry instrumentation.

You can validate if your application is sending traces to Hanzo O11y cloud by following the instructions [here](https://o11y.hanzo.ai/docs/instrumentation/tomcat/#validating-instrumentation-by-checking-for-traces).