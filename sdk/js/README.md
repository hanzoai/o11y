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

## Status: encoding done, transport blocked

`encodeSpanBatch` is complete and correct. Delivery is not.

`pkg/zapreceiver` is a ZAP **node**: the Go emitter reaches it with
`node.ConnectDirect(endpoint)` and sends to the resolved peer ID. That node/peer
layer lives in `luxfi/zap`. The published TypeScript runtime, `@zap-proto/zap`
1.6.0, is **wire format only** — `Builder`, `Message`, envelopes, `Pipeliner` —
with no node, client, connect or peer export. Writing a correct frame onto a bare
TCP socket, which is what this SDK does today, is not the protocol: the receiver
accepts the connection and decodes nothing.

So this package encodes correctly and its spans do not arrive. Closing it needs
the node/peer transport in `@zap-proto/zap` (handshake, peer identity, framed
send), at which point `O11y.send` swaps its socket write for a peer send and
nothing else here changes.

Not published until then.
