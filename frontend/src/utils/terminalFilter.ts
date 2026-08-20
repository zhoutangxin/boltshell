/**
 * 终端输出/输入过滤纯函数（可单元测试）
 */

/** xterm 自动应答（不应发回 SSH） */
export const XTERM_AUTO_RESPONSE =
  /^\x1b(\[[?0-9;]*c|\[[0-9]+;[0-9]+R|\[>[?0-9;]*c|\[[0-9]+;[0-9]+;[0-9]+t)/

/** 完整 CSI / SS3 序列（用户方向键等） */
export const CSI_OR_SS3 = /^\x1b(\[[\x3f0-9;]*[\x40-\x7e]|[\x20-\x7e])/

/** 服务器 stdout 中会导致 xterm 自动应答 / resize 的 CSI（echo 循环根源） */
const INBOUND_CSI_STRIP =
  /\x1b\[[0-9;?]*[0-9]*[cRt]|\x1b\[>[?0-9;]*c|\x1b\[\?[0-9;]+[0-9]+n|\x1b\[6n|\x1b\[(18|19|14)t/g

/** 过滤 SSH 输出中的乱码与危险 CSI，再写入 xterm */
export function sanitizeTerminalOutput(data: string): string {
  if (!data) return data
  return data
    .replace(INBOUND_CSI_STRIP, '')
    .replace(/\x1b\[[?0-9;]*c/g, '')
    .replace(/\x1b\[>[?0-9;]*c/g, '')
    .replace(/\x1b\[[0-9]+;[0-9]+R/g, '')
    .replace(/\x1b\[[0-9]+;[0-9]+;[0-9]+t/g, '')
    .replace(/[wW]{12,}/g, '')
    .replace(/(\r\n){2,}/g, '\r\n')
}

/** 跨 chunk 拼接后再过滤（避免 EventsEmit 拆包导致 CSI 漏网） */
export function sanitizeTerminalChunk(
  sessionID: string,
  chunk: string,
  tails: Map<string, string>,
): string {
  let data = (tails.get(sessionID) ?? '') + chunk
  tails.set(sessionID, '')

  const hold = data.match(/\x1b(?:\[[\x30-\x3f]*|[\x20-\x7e]?)$/)
  if (hold) {
    tails.set(sessionID, hold[0])
    data = data.slice(0, -hold[0].length)
  }
  return sanitizeTerminalOutput(data)
}

export type OutboundDrainResult = {
  toSend: string[]
  remaining: string
}

/** 从 onData 缓冲中拆出应发往 SSH 的用户输入，丢弃 xterm 自动应答 */
export function drainTerminalOutbound(outbound: string): OutboundDrainResult {
  const toSend: string[] = []
  let buf = outbound

  while (buf.length > 0) {
    if (buf.charCodeAt(0) === 0x1b) {
      const auto = buf.match(XTERM_AUTO_RESPONSE)
      if (auto) {
        buf = buf.slice(auto[0].length)
        continue
      }
      if (buf.length < 64 && /^\x1b(\[[\x30-\x3f]*$)/.test(buf)) break
    }

    const escIdx = buf.indexOf('\x1b')
    if (escIdx === -1) {
      if (buf) toSend.push(buf)
      return { toSend, remaining: '' }
    }
    if (escIdx > 0) {
      toSend.push(buf.slice(0, escIdx))
      buf = buf.slice(escIdx)
      continue
    }

    const seq = buf.match(CSI_OR_SS3)
    if (seq) {
      toSend.push(seq[0])
      buf = buf.slice(seq[0].length)
      continue
    }
    break
  }

  return { toSend, remaining: buf }
}

/** 是否像 xterm 自动 CSI 应答（用于断言） */
export function isXtermAutoResponse(data: string): boolean {
  return XTERM_AUTO_RESPONSE.test(data)
}

/** 注册与 composable 一致的 CSI 处理器 */
export function registerTerminalCsiHandlers(term: {
  parser: {
    registerCsiHandler: (
      id: { prefix?: string; final: string },
      cb: (params: number[]) => boolean,
    ) => unknown
  }
}): void {
  term.parser.registerCsiHandler({ final: 'c' }, () => true)
  term.parser.registerCsiHandler({ prefix: '>', final: 'c' }, () => true)
  term.parser.registerCsiHandler({ final: 'n' }, (params) => params.length > 0 && params[0] === 6)
  term.parser.registerCsiHandler({ final: 't' }, (params) => {
    if (params.length > 0 && (params[0] === 8 || params[0] === 18 || params[0] === 19 || params[0] === 14)) {
      return true
    }
    return false
  })
}
