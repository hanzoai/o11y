import { Builder } from '@zap-proto/zap'

/**
 * The wire shapes below mirror luxfi/trace.SpanBatch field for field. The Go
 * emitter, this emitter and hanzo/o11y/pkg/zapreceiver all read the same JSON,
 * so a field added on one side must be added on the others in the same change.
 */

export interface SpanEvent {
  name: string
  timeUnixNs: number
  attributes?: Record<string, unknown>
}

export interface Span {
  traceId: string
  spanId: string
  parentSpanId?: string
  name: string
  kind?: string
  startUnixNs: number
  endUnixNs: number
  attributes?: Record<string, unknown>
  events?: SpanEvent[]
  statusCode?: string
  statusMessage?: string
}

export interface SpanBatch {
  appName?: string
  version?: string
  resource?: Record<string, string>
  spans: Span[]
}

/**
 * The ZAP message-type slot for a span batch. Stable and append-only:
 * renumbering it breaks every deployed collector at once.
 */
export const MSG_SPAN_BATCH = 1

/** luxfi/zap emits transport envelopes at header version 2. */
const TRANSPORT_VERSION = 2

/** The root object carries one field: the payload bytes at offset 0. */
const ROOT_DATA_SIZE = 8

/**
 * Encode a batch as the single frame the collector expects: a luxfi/zap
 * envelope whose header flags carry the message type in their high byte, with
 * JSON in the root object's first field.
 *
 * Flags rather than a dedicated header word is luxfi/zap's own layout — Go
 * writes `FinishWithFlags(MsgSpanBatch << 8)` and this must match it byte for
 * byte, because the collector reads the type back out of the same bits.
 */
export function encodeSpanBatch(batch: SpanBatch): Uint8Array {
  const payload = new TextEncoder().encode(JSON.stringify(batch))
  const b = new Builder(payload.length + 128, TRANSPORT_VERSION)
  const root = b.startObject(ROOT_DATA_SIZE)
  root.setBytes(0, payload)
  root.finishAsRoot()
  return b.finishWithFlags(MSG_SPAN_BATCH << 8)
}
