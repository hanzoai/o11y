import { connect, type Socket } from 'node:net'
import { Builder, Message, StructView } from '@zap-proto/zap'

/**
 * The luxfi/zap node transport: length-prefixed frames over TCP, opened by a
 * NodeID exchange.
 *
 * A collector is a ZAP node, not a socket that accepts loose frames. Until the
 * handshake completes it discards what it receives, so this layer is what makes
 * a correctly-encoded span batch actually arrive.
 */

/** node_codec.go: the ID occupies bytes [0,60), its length sits at offset 60. */
const MAX_NODE_ID_LEN = 60
const HANDSHAKE_OBJECT_SIZE = 64

/** luxfi/zap emits at header version 2 (Go's NewBuilder default). */
const VERSION_2 = 2

/** Frames are prefixed with a little-endian uint32 length. */
const LENGTH_PREFIX_BYTES = 4

/** Refuse absurd lengths rather than allocating whatever a peer claims. */
const MAX_FRAME_BYTES = 64 * 1024 * 1024

function encodeNodeIDHandshake(nodeID: string): Uint8Array {
  const id = new TextEncoder().encode(nodeID).subarray(0, MAX_NODE_ID_LEN)
  const b = new Builder(128, VERSION_2)
  const obj = b.startObject(HANDSHAKE_OBJECT_SIZE)
  for (let i = 0; i < id.length; i++) obj.setU8(i, id[i]!)
  obj.setU32(MAX_NODE_ID_LEN, id.length)
  obj.finishAsRoot()
  return b.finish()
}

/**
 * StructView's accessors are protected — it is the base that generated views
 * extend, and a handshake has no generated view. This is that view.
 */
class HandshakeView extends StructView {
  nodeID(): string | null {
    const len = this.u32(MAX_NODE_ID_LEN)
    if (len === 0 || len > MAX_NODE_ID_LEN) return null
    const id = new Uint8Array(len)
    for (let i = 0; i < len; i++) id[i] = this.u8(i)
    return new TextDecoder().decode(id)
  }
}

function decodeNodeIDHandshake(data: Uint8Array): string | null {
  try {
    // Message.root() hands back the base view; re-seat it at the same bytes and
    // offset as the typed one, which is what a generated view does too.
    const root = Message.parse(data).root() as unknown as {
      data: Uint8Array
      offset: number
    }
    return new HandshakeView(root.data, root.offset).nodeID()
  } catch {
    return null
  }
}

function frame(payload: Uint8Array): Uint8Array {
  const out = new Uint8Array(LENGTH_PREFIX_BYTES + payload.length)
  new DataView(out.buffer).setUint32(0, payload.length, true)
  out.set(payload, LENGTH_PREFIX_BYTES)
  return out
}

/** Read exactly one length-prefixed frame. */
function readFrame(socket: Socket, timeoutMs: number): Promise<Uint8Array> {
  return new Promise((resolve, reject) => {
    let buf = Buffer.alloc(0)
    const timer = setTimeout(() => finish(new Error('zap: handshake timed out')), timeoutMs)
    const onData = (chunk: Buffer) => {
      buf = Buffer.concat([buf, chunk])
      if (buf.length < LENGTH_PREFIX_BYTES) return
      const len = buf.readUInt32LE(0)
      if (len > MAX_FRAME_BYTES) return finish(new Error(`zap: frame too large (${len})`))
      if (buf.length < LENGTH_PREFIX_BYTES + len) return
      finish(null, new Uint8Array(buf.subarray(LENGTH_PREFIX_BYTES, LENGTH_PREFIX_BYTES + len)))
    }
    const finish = (err: Error | null, value?: Uint8Array) => {
      clearTimeout(timer)
      socket.off('data', onData)
      socket.off('error', onError)
      err ? reject(err) : resolve(value!)
    }
    const onError = (err: Error) => finish(err)
    socket.on('data', onData)
    socket.once('error', onError)
  })
}

/** A connected ZAP peer. */
export interface Peer {
  /** The remote node's ID, learned from its handshake. */
  readonly peerID: string
  /** Send one already-encoded ZAP message. Fire-and-forget. */
  send(message: Uint8Array): Promise<void>
  close(): void
  readonly closed: boolean
}

export interface DialOptions {
  host: string
  port: number
  /** Our own node ID, sent in the handshake. */
  nodeID: string
  timeoutMs?: number
}

/** Dial a ZAP node and complete the NodeID exchange. */
export async function dial(opts: DialOptions): Promise<Peer> {
  const timeoutMs = opts.timeoutMs ?? 5_000
  const socket = await new Promise<Socket>((resolve, reject) => {
    const s = connect({ host: opts.host, port: opts.port })
    s.setNoDelay(true)
    // Telemetry should never be the reason a process stays alive.
    s.unref()
    const timer = setTimeout(() => {
      s.destroy()
      reject(new Error(`zap: connect to ${opts.host}:${opts.port} timed out`))
    }, timeoutMs)
    s.once('connect', () => {
      clearTimeout(timer)
      resolve(s)
    })
    s.once('error', (err) => {
      clearTimeout(timer)
      reject(err)
    })
  })

  await new Promise<void>((resolve, reject) => {
    socket.write(frame(encodeNodeIDHandshake(opts.nodeID)), (err) =>
      err ? reject(err) : resolve(),
    )
  })

  const peerID = decodeNodeIDHandshake(await readFrame(socket, timeoutMs))
  if (peerID === null) {
    socket.destroy()
    throw new Error('zap: invalid peer handshake')
  }

  let closed = false
  socket.once('close', () => {
    closed = true
  })

  return {
    peerID,
    get closed() {
      return closed || socket.destroyed
    },
    send: (message) =>
      new Promise<void>((resolve, reject) => {
        socket.write(frame(message), (err) => (err ? reject(err) : resolve()))
      }),
    close: () => {
      closed = true
      socket.end()
    },
  }
}
