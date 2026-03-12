<h1 align="center" style="border-bottom: none">
    <a href="https://o11y.hanzo.ai" target="_blank">
        <img alt="Hanzo O11y" src="https://github.com/user-attachments/assets/ef9a33f7-12d7-4c94-8908-0a02b22f0c18" width="100" height="100">
    </a>
    <br>Hanzo O11y
</h1>

<p align="center">All your logs, metrics, and traces in one place. Monitor your application, spot issues before they occur and troubleshoot downtime quickly with rich context. Hanzo O11y is a cost-effective open-source alternative to Datadog and New Relic. Visit <a href="https://o11y.hanzo.ai" target="_blank">o11y.hanzo.ai</a> for the full documentation, tutorials, and guide.</p>

<p align="center">
    <img alt="GitHub issues" src="https://img.shields.io/github/issues/signoz/signoz"> </a>
    <a href="https://twitter.com/intent/tweet?text=Monitor%20your%20applications%20and%20troubleshoot%20problems%20with%20Hanzo O11y,%20an%20open-source%20alternative%20to%20DataDog,%20NewRelic.&url=https://o11y.hanzo.ai/&via=Hanzo O11yHQ&hashtags=opensource,signoz,observability"> 
        <img alt="tweet" src="https://img.shields.io/twitter/url/http/shields.io.svg?style=social"> </a> 
</p>
  
  
<h3 align="center">
  <a href="https://o11y.hanzo.ai/docs"><b>Documentation</b></a> &bull;
  <a href="https://github.com/Hanzo O11y/signoz/blob/main/README.zh-cn.md"><b>ReadMe in Chinese</b></a> &bull;
  <a href="https://github.com/Hanzo O11y/signoz/blob/main/README.de-de.md"><b>ReadMe in German</b></a> &bull;
  <a href="https://github.com/Hanzo O11y/signoz/blob/main/README.pt-br.md"><b>ReadMe in Portuguese</b></a> &bull;
  <a href="https://o11y.hanzo.ai/slack"><b>Slack Community</b></a> &bull;
  <a href="https://twitter.com/Hanzo O11yHq"><b>Twitter</b></a>
</h3>

## Features


### Application Performance Monitoring

Use Hanzo O11y APM to monitor your applications and services. It comes with out-of-box charts for key application metrics like p99 latency, error rate, Apdex and operations per second. You can also monitor the database and external calls made from your application. Read [more](https://o11y.hanzo.ai/application-performance-monitoring/).

You can [instrument](https://o11y.hanzo.ai/docs/instrumentation/) your application with OpenTelemetry to get started.

![apm-cover](https://github.com/user-attachments/assets/fa5c0396-0854-4c8b-b972-9b62fd2a70d2)


### Logs Management

Hanzo O11y can be used as a centralized log management solution. We use ClickHouse (used by likes of Uber & Cloudflare) as a datastore, ⎯ an extremely fast and highly optimized storage for logs data. Instantly search through all your logs using quick filters and a powerful query builder.

You can also create charts on your logs and monitor them with customized dashboards. Read [more](https://o11y.hanzo.ai/log-management/).

![logs-management-cover](https://github.com/user-attachments/assets/343588ee-98fb-4310-b3d2-c5bacf9c7384)


### Distributed Tracing

Distributed Tracing is essential to troubleshoot issues in microservices applications. Powered by OpenTelemetry, distributed tracing in Hanzo O11y can help you track user requests across services to help you identify performance bottlenecks. 

See user requests in a detailed breakdown with the help of Flamegraphs and Gantt Charts. Click on any span to see the entire trace represented beautifully, which will help you make sense of where issues actually occurred in the flow of requests.

Read [more](https://o11y.hanzo.ai/distributed-tracing/).

![distributed-tracing-cover](https://github.com/user-attachments/assets/9bfe060a-0c40-4922-9b55-8a97e1a4076c)



### Metrics and Dashboards

Ingest metrics from your infrastructure or applications and create customized dashboards to monitor them. Create visualization that suits your needs with a variety of panel types like pie chart, time-series, bar chart, etc.

Create queries on your metrics data quickly with an easy-to-use metrics query builder. Add multiple queries and combine those queries with formulae to create really complex queries quickly.

Read [more](https://o11y.hanzo.ai/metrics-and-dashboards/).

![metrics-n-dashboards-cover](https://github.com/user-attachments/assets/a536fd71-1d2c-4681-aa7e-516d754c47a5)

### LLM Observability

Monitor and debug your LLM applications with comprehensive observability. Track LLM calls, analyze token usage, monitor performance, and gain insights into your AI application's behavior in production.

Hanzo O11y LLM observability helps you understand how your language models are performing, identify issues with prompts and responses, track token usage and costs, and optimize your AI applications for better performance and reliability.

[Get started with LLM Observability →](https://o11y.hanzo.ai/docs/llm-observability/)

![llm-observability-cover](https://github.com/user-attachments/assets/a6cc0ca3-59df-48f9-9c16-7c843fccff96)


### Alerts

Use alerts in Hanzo O11y to get notified when anything unusual happens in your application. You can set alerts on any type of telemetry signal (logs, metrics, traces), create thresholds and set up a notification channel to get notified. Advanced features like alert history and anomaly detection can help you create smarter alerts.

Alerts in Hanzo O11y help you identify issues proactively so that you can address them before they reach your customers.

Read [more](https://o11y.hanzo.ai/alerts-management/).

![alerts-cover](https://github.com/user-attachments/assets/03873bb8-1b62-4adf-8f56-28bb7b1750ea)

### Exceptions Monitoring

Monitor exceptions automatically in Python, Java, Ruby, and Javascript. For other languages, just drop in a few lines of code and start monitoring exceptions.

See the detailed stack trace for all exceptions caught in your application. You can also log in custom attributes to add more context to your exceptions. For example, you can add attributes to identify users for which exceptions occurred.

Read [more](https://o11y.hanzo.ai/exceptions-monitoring/).


![exceptions-cover](https://github.com/user-attachments/assets/4be37864-59f2-4e8a-8d6e-e29ad04298c5)


<br /><br />

## Why Hanzo O11y?

Hanzo O11y is a single tool for all your monitoring and observability needs. Here are a few reasons why you should choose Hanzo O11y:

- Single tool for observability(logs, metrics, and traces)

- Built on top of [OpenTelemetry](https://opentelemetry.io/), the open-source standard which frees you from any type of vendor lock-in

- Correlated logs, metrics and traces for much richer context while debugging

- Uses ClickHouse (used by likes of Uber & Cloudflare) as datastore - an extremely fast and highly optimized storage for observability data

- DIY Query builder, PromQL, and ClickHouse queries to fulfill all your use-cases around querying observability data

- Open-Source - you can use open-source, our [cloud service](https://o11y.hanzo.ai/teams/) or a mix of both based on your use case


## Getting Started

### Create a Hanzo O11y Cloud Account

Hanzo O11y cloud is the easiest way to get started with Hanzo O11y. Our cloud service is for those users who want to spend more time in getting insights for their application performance without worrying about maintenance. 

[Get started for free](https://o11y.hanzo.ai/teams/)

### Deploy using Docker(self-hosted)

Please follow the steps listed [here](https://o11y.hanzo.ai/docs/install/docker/) to install using docker

The [troubleshooting instructions](https://o11y.hanzo.ai/docs/install/troubleshooting/) may be helpful if you face any issues.

<p>&nbsp  </p>
  
  
### Deploy in Kubernetes using Helm(self-hosted)

Please follow the steps listed [here](https://o11y.hanzo.ai/docs/deployment/helm_chart) to install using helm charts

<br /><br />

We also offer managed services in your infra. Check our [pricing plans](https://o11y.hanzo.ai/pricing/) for all details.


## Join our Slack community

Come say Hi to us on [Slack](https://o11y.hanzo.ai/slack) 👋

<br /><br />


### Languages supported:

Hanzo O11y supports all major programming languages for monitoring. Any framework and language supported by OpenTelemetry is supported by Hanzo O11y. Find instructions for instrumenting different languages below:

- [Java](https://o11y.hanzo.ai/docs/instrumentation/java/)
- [Python](https://o11y.hanzo.ai/docs/instrumentation/python/)
- [Node.js or Javascript](https://o11y.hanzo.ai/docs/instrumentation/javascript/)
- [Go](https://o11y.hanzo.ai/docs/instrumentation/golang/)
- [PHP](https://o11y.hanzo.ai/docs/instrumentation/php/)
- [.NET](https://o11y.hanzo.ai/docs/instrumentation/dotnet/)
- [Ruby](https://o11y.hanzo.ai/docs/instrumentation/ruby-on-rails/)
- [Elixir](https://o11y.hanzo.ai/docs/instrumentation/elixir/)
- [Rust](https://o11y.hanzo.ai/docs/instrumentation/rust/)
- [Swift](https://o11y.hanzo.ai/docs/instrumentation/swift/)

You can find our entire documentation [here](https://o11y.hanzo.ai/docs/introduction/).

<br /><br />


## Comparisons to Familiar Tools

### Hanzo O11y vs Prometheus

Prometheus is good if you want to do just metrics. But if you want to have a seamless experience between metrics, logs and traces, then current experience of stitching together Prometheus & other tools is not great.

Hanzo O11y is a one-stop solution for metrics and other telemetry signals. And because you will use the same standard(OpenTelemetry) to collect all telemetry signals, you can also correlate these signals to troubleshoot quickly.

For example, if you see that there are issues with infrastructure metrics of your k8s cluster at a timestamp, you can jump to other signals like logs and traces to understand the issue quickly.

<p>&nbsp  </p>

### Hanzo O11y vs Jaeger

Jaeger only does distributed tracing. Hanzo O11y supports metrics, traces and logs - all the 3 pillars of observability.

Moreover, Hanzo O11y has few more advanced features wrt Jaeger:

- Jaegar UI doesn’t show any metrics on traces or on filtered traces
- Jaeger can’t get aggregates on filtered traces. For example, p99 latency of requests which have tag - customer_type='premium'. This can be done easily on Hanzo O11y
- You can also go from traces to logs easily in Hanzo O11y

<p>&nbsp  </p>

### Hanzo O11y vs Elastic 

- Hanzo O11y Logs management are based on ClickHouse, a columnar OLAP datastore which makes aggregate log analytics queries much more efficient
- 50% lower resource requirement compared to Elastic during ingestion

We have published benchmarks comparing Elastic with Hanzo O11y. Check it out [here](https://o11y.hanzo.ai/blog/logs-performance-benchmark/?utm_source=github-readme&utm_medium=logs-benchmark)

<p>&nbsp  </p>

### Hanzo O11y vs Loki

- Hanzo O11y supports aggregations on high-cardinality data over a huge volume while loki doesn’t.
- Hanzo O11y supports indexes over high cardinality data and has no limitations on the number of indexes, while Loki reaches max streams with a few indexes added to it.
- Searching over a huge volume of data is difficult and slow in Loki compared to Hanzo O11y

We have published benchmarks comparing Loki with Hanzo O11y. Check it out [here](https://o11y.hanzo.ai/blog/logs-performance-benchmark/?utm_source=github-readme&utm_medium=logs-benchmark)

<br /><br />


## Contributing

We ❤️ contributions big or small. Please read [CONTRIBUTING.md](CONTRIBUTING.md) to get started with making contributions to Hanzo O11y.

Not sure how to get started? Just ping us on `#contributing` in our [slack community](https://o11y.hanzo.ai/slack)

<br /><br />


## Documentation

You can find docs at https://o11y.hanzo.ai/docs/. If you need any clarification or find something missing, feel free to raise a GitHub issue with the label `documentation` or reach out to us at the community slack channel.

<br /><br />


## Community

Join the [slack community](https://o11y.hanzo.ai/slack) to know more about distributed tracing, observability, or Hanzo O11y and to connect with other users and contributors.

If you have any ideas, questions, or any feedback, please share on our [Github Discussions](https://github.com/Hanzo O11y/signoz/discussions)

As always, thanks to our amazing contributors!

<a href="https://github.com/signoz/signoz/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=signoz/signoz" />
</a>
