import { randomBytes } from 'node:crypto'

import { dial, type Peer } from './transport.js'
import { encodeSpanBatch, type Span, type SpanBatch } from './wire.js'

export { encodeSpanBatch, MSG_SPAN_BATCH } from './wire.js'
export type { Span, SpanBatch, SpanEvent } from './wire.js'
export { dial } from './transport.js'
export type { Peer } from './transport.js'

export interface O11yOptions {
  /** Service name recorded on every span. */
  appName: string
  version?: string
  /** Collector address. Defaults to o11y's canonical ZAP port. */
  host?: string
  port?: number
  /** Resource attributes attached to every batch. */
  resource?: Record<string, string>
  /** Flush when this many spans are queued. */
  batchSize?: number
  /** Flush at least this often, in milliseconds. */
  flushIntervalMs?: number
  /** Our node ID in the ZAP handshake. Defaults to the app name. */
  nodeID?: string
}

/** A span in progress. Ending it queues it for export. */
export interface ActiveSpan {
  readonly traceId: string
  readonly spanId: string
  setAttribute(key: string, value: unknown): void
  addEvent(name: string, attributes?: Record<string, unknown>): void
  setStatus(code: 'ok' | 'error', message?: string): void
  end(): void
}

const nowUnixNs = (): number => Math.round(performance.timeOrigin * 1e6 + performance.now() * 1e6)
const hex = (bytes: number): string => randomBytes(bytes).toString('hex')

/**
 * Collects spans and ships them to o11y over ZAP.
 *
 * There is no OpenTelemetry SDK underneath and nothing translates between two
 * span models: a span here is already the shape that goes on the wire, so the
 * only work at export time is JSON plus a frame.
 */
export class O11y {
  private readonly opts: O11yOptions &
    Required<Pick<O11yOptions, 'host' | 'port' | 'batchSize' | 'flushIntervalMs'>>
  private queue: Span[] = []
  private peer: Peer | null = null
  private timer: NodeJS.Timeout | null = null
  private closed = false

  constructor(options: O11yOptions) {
    this.opts = {
      host: '127.0.0.1',
      port: 4317,
      batchSize: 512,
      flushIntervalMs: 5_000,
      ...options,
    }
    this.timer = setInterval(() => void this.flush(), this.opts.flushIntervalMs)
    // A telemetry timer is not a reason to keep a process alive.
    this.timer.unref?.()
  }

  /**
   * Start a span. Pass `parent` to nest; omit it to start a new trace.
   */
  startSpan(name: string, parent?: { traceId: string; spanId: string }): ActiveSpan {
    const span: Span = {
      traceId: parent?.traceId ?? hex(16),
      spanId: hex(8),
      parentSpanId: parent?.spanId,
      name,
      startUnixNs: nowUnixNs(),
      endUnixNs: 0,
    }
    let ended = false
    return {
      traceId: span.traceId,
      spanId: span.spanId,
      setAttribute: (key, value) => {
        ;(span.attributes ??= {})[key] = value
      },
      addEvent: (eventName, attributes) => {
        ;(span.events ??= []).push({ name: eventName, timeUnixNs: nowUnixNs(), attributes })
      },
      setStatus: (code, message) => {
        span.statusCode = code
        span.statusMessage = message
      },
      end: () => {
        // Ending twice would double-count the span; the second call is a no-op.
        if (ended) return
        ended = true
        span.endUnixNs = nowUnixNs()
        this.enqueue(span)
      },
    }
  }

  private enqueue(span: Span): void {
    if (this.closed) return
    this.queue.push(span)
    if (this.queue.length >= this.opts.batchSize) void this.flush()
  }

  /** Send everything queued. Safe to call at any time; a no-op when empty. */
  async flush(): Promise<void> {
    if (this.queue.length === 0) return
    const spans = this.queue
    this.queue = []
    const batch: SpanBatch = {
      appName: this.opts.appName,
      version: this.opts.version,
      resource: this.opts.resource,
      spans,
    }
    try {
      await this.send(encodeSpanBatch(batch))
    } catch {
      // Telemetry must not take the application down with it, and a failed
      // batch is not worth retrying into an unbounded queue: drop and continue.
    }
  }

  private async send(frame: Uint8Array): Promise<void> {
    const peer = await this.peerConn()
    await peer.send(frame)
  }

  private async peerConn(): Promise<Peer> {
    if (this.peer && !this.peer.closed) return this.peer
    this.peer = await dial({
      host: this.opts.host,
      port: this.opts.port,
      nodeID: this.opts.nodeID ?? this.opts.appName,
    })
    return this.peer
  }

  /** Flush what is queued and stop. The instance is unusable afterwards. */
  async shutdown(): Promise<void> {
    if (this.closed) return
    this.closed = true
    if (this.timer) clearInterval(this.timer)
    // Closed blocks enqueue, so drain what arrived before this call.
    const pending = this.queue
    this.queue = pending
    this.closed = false
    await this.flush()
    this.closed = true
    this.peer?.close()
    this.peer = null
  }
}
