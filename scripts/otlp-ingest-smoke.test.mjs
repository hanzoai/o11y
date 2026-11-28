import test from 'node:test';
import assert from 'node:assert/strict';
import http from 'node:http';

function startMockOtlpCollector() {
  const received = { traces: [], metrics: [], logs: [] };
  const server = http.createServer(async (req, res) => {
    const chunks = [];
    for await (const chunk of req) chunks.push(chunk);
    const body = JSON.parse(Buffer.concat(chunks).toString('utf8'));
    res.setHeader('Content-Type', 'application/json');

    if (req.method === 'POST' && req.url === '/v1/traces') {
      received.traces.push(body);
      res.end(JSON.stringify({ partialSuccess: {} }));
      return;
    }
    if (req.method === 'POST' && req.url === '/v1/metrics') {
      received.metrics.push(body);
      res.end(JSON.stringify({ partialSuccess: {} }));
      return;
    }
    if (req.method === 'POST' && req.url === '/v1/logs') {
      received.logs.push(body);
      res.end(JSON.stringify({ partialSuccess: {} }));
      return;
    }
    res.statusCode = 404;
    res.end(JSON.stringify({ error: 'Not found' }));
  });

  return new Promise((resolve) => {
    server.listen(0, '127.0.0.1', () => {
      const { port } = server.address();
      resolve({ server, received, baseUrl: `http://127.0.0.1:${port}` });
    });
  });
}

test('OTLP HTTP/JSON collector accepts traces, metrics, and logs', async () => {
  const { server, received, baseUrl } = await startMockOtlpCollector();
  try {
    for (const [signal, payload] of [
      ['traces', { resourceSpans: [{ scopeSpans: [{ spans: [{ name: 'hanzo.cloud.http' }] }] }] }],
      ['metrics', { resourceMetrics: [{ scopeMetrics: [{ metrics: [{ name: 'hanzo.cloud.requests' }] }] }] }],
      ['logs', { resourceLogs: [{ scopeLogs: [{ logRecords: [{ body: { stringValue: 'ready' } }] }] }] }],
    ]) {
      const res = await fetch(`${baseUrl}/v1/${signal}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      });
      assert.equal(res.status, 200);
    }

    assert.equal(received.traces.length, 1);
    assert.equal(received.metrics.length, 1);
    assert.equal(received.logs.length, 1);
  } finally {
    await new Promise((resolve) => server.close(resolve));
  }
});
