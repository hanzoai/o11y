# @hanzo/o11y

Traces to Hanzo o11y over ZAP. No OpenTelemetry, no OTLP, no gRPC, no protobuf.

```ts
import { O11y } from '@hanzo/o11y'

const o11y = new O11y({ appName: 'my-service', host: 'o11y.hanzo.svc' })

const span = o11y.startSpan('handle-request')
span.setAttribute('http.method', 'GET')
const child = o11y.startSpan('db-query', span)
child.end()
span.end()

await o11y.shutdown()
```

A span here is already the shape that goes on the wire, so export is JSON plus a
frame — there is no second span model to translate through.

## Wire

`src/wire.ts` mirrors `luxfi/trace.SpanBatch` field for field, and the frame
matches what `hanzo/o11y/pkg/zapreceiver` decodes: a luxfi/zap envelope at header
version 2, message type in the flags high byte (`MSG_SPAN_BATCH << 8`), JSON in
the root object's first field. Verified byte-identical to the Go emitter's
`FinishWithFlags(MsgSpanBatch << 8)`.

Adding a field means adding it to all three — this SDK, `luxfi/trace`, and
`pkg/zapreceiver` — in one change.

## Transport

The collector is a ZAP node, not a socket that takes loose frames — until the
NodeID exchange completes it discards what arrives. `src/transport.ts` is that
layer: length-prefixed frames over TCP (little-endian uint32), opened by the
handshake `luxfi/zap` defines in `node_codec.go` — the ID in bytes [0,60) and its
length at offset 60.

`@zap-proto/zap` supplies the wire format and stops there, so the node/peer layer
lives here. If it grows one, this file is what to delete.

Verified end to end against a live `pkg/zapreceiver`: spans sent from Node arrive
decoded, with trace/parent linkage, attributes, events and status intact.
