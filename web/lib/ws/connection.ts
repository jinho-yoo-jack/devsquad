/**
 * 13-Frontend 설계 §4 — WebSocket 연결 관리.
 *
 * - 지수 백오프 재연결 (1s → 2s → 4s … 최대 30s, jitter)
 * - 재연결 성공 시 구독 중인 task 를 lastSeq 기준 from_seq 로 재구독 → 서버가 replay
 * - heartbeat: ping 을 보내고 pongTimeout 안에 pong 이 없으면 끊고 재연결
 * - seq 구멍 감지 시 onGap 콜백 → 호출자가 REST 로 채운다
 *
 * 브라우저 API 에만 의존하고 React 를 모른다. 테스트는 mock WebSocket 으로.
 */
import { TaskEvent } from "@/lib/domain/events";
import { detectGap } from "@/lib/stores/eventStore";

export type ServerMessage =
  | { op: "pong" }
  | { op: "subscribed"; task_id?: string; replayed: number }
  | { op: "error"; code: string; message?: string }
  | TaskEvent;

export type ConnectionOptions = {
  url: string;
  /** 구독 중인 task 의 마지막 seq 를 돌려준다 (재구독 from_seq 용). */
  getLastSeq: (taskId: string) => number;
  onEvent: (ev: TaskEvent) => void;
  onGap?: (taskId: string, fromSeq: number, toSeq: number) => void;
  onStatus?: (status: "connecting" | "open" | "reconnecting" | "closed", attempt: number) => void;
  onServerError?: (code: string, message?: string) => void;
  /** 테스트 주입용. 기본은 전역 WebSocket. */
  wsFactory?: (url: string) => WebSocket;
  baseDelayMs?: number;
  maxDelayMs?: number;
  pingIntervalMs?: number;
  pongTimeoutMs?: number;
  now?: () => number;
  random?: () => number;
};

export class WsConnection {
  private ws: WebSocket | null = null;
  private subscriptions = new Set<string>(); // task_id; "" 는 요약 채널
  private attempt = 0;
  private closedByUser = false;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private pingTimer: ReturnType<typeof setInterval> | null = null;
  private pongTimer: ReturnType<typeof setTimeout> | null = null;
  private readonly opts: Required<Pick<ConnectionOptions, "baseDelayMs" | "maxDelayMs" | "pingIntervalMs" | "pongTimeoutMs" | "random">> & ConnectionOptions;

  constructor(opts: ConnectionOptions) {
    this.opts = {
      baseDelayMs: 1000,
      maxDelayMs: 30_000,
      pingIntervalMs: 15_000,
      pongTimeoutMs: 30_000,
      random: Math.random,
      ...opts,
    };
  }

  get isOpen(): boolean {
    return this.ws?.readyState === 1; // WebSocket.OPEN
  }

  connect(): void {
    this.closedByUser = false;
    this.open();
  }

  close(): void {
    this.closedByUser = true;
    this.clearTimers();
    this.ws?.close();
    this.ws = null;
    this.opts.onStatus?.("closed", this.attempt);
  }

  subscribe(taskId: string, fromSeq?: number): void {
    this.subscriptions.add(taskId);
    if (this.isOpen) this.send({ op: "subscribe", ...(taskId ? { task_id: taskId } : {}), ...(fromSeq !== undefined ? { from_seq: fromSeq } : {}) });
  }

  unsubscribe(taskId: string): void {
    this.subscriptions.delete(taskId);
    if (this.isOpen) this.send({ op: "unsubscribe", task_id: taskId });
  }

  /** 다음 재연결까지의 지연. 테스트 가능하게 public. */
  nextDelayMs(attempt: number): number {
    const exp = Math.min(this.opts.maxDelayMs, this.opts.baseDelayMs * 2 ** Math.max(0, attempt - 1));
    const jitter = exp * 0.2 * (this.opts.random() * 2 - 1); // ±20%
    return Math.max(this.opts.baseDelayMs / 2, Math.round(exp + jitter));
  }

  // ---- 내부 -------------------------------------------------------------

  private open(): void {
    this.clearTimers();
    this.opts.onStatus?.(this.attempt === 0 ? "connecting" : "reconnecting", this.attempt);
    const factory = this.opts.wsFactory ?? ((u: string) => new WebSocket(u));
    const ws = factory(this.opts.url);
    this.ws = ws;

    ws.onopen = () => {
      this.attempt = 0;
      this.opts.onStatus?.("open", 0);
      // 재구독: 각 task 는 마지막 seq 부터 replay 받는다
      for (const taskId of this.subscriptions) {
        const last = taskId ? this.opts.getLastSeq(taskId) : 0;
        this.send({ op: "subscribe", ...(taskId ? { task_id: taskId } : {}), ...(last > 0 ? { from_seq: last } : {}) });
      }
      this.startHeartbeat();
    };

    ws.onmessage = (msg: MessageEvent) => {
      let data: unknown;
      try {
        data = JSON.parse(typeof msg.data === "string" ? msg.data : String(msg.data));
      } catch {
        return;
      }
      this.handle(data);
    };

    ws.onclose = () => {
      this.clearTimers();
      if (this.closedByUser) return;
      this.scheduleReconnect();
    };

    ws.onerror = () => {
      // onclose 가 이어서 호출된다. 여기서는 아무것도 하지 않는다.
    };
  }

  private handle(data: unknown): void {
    if (!data || typeof data !== "object") return;
    const d = data as Record<string, unknown>;
    if (d.op === "pong") {
      if (this.pongTimer) clearTimeout(this.pongTimer);
      this.pongTimer = null;
      return;
    }
    if (d.op === "subscribed") return;
    if (d.op === "error") {
      this.opts.onServerError?.(String(d.code), typeof d.message === "string" ? d.message : undefined);
      return;
    }
    const parsed = TaskEvent.safeParse(d);
    if (!parsed.success) return; // 알 수 없는 형태는 무시 (서버가 타입을 추가해도 안 깨짐)
    const ev = parsed.data;
    if (this.subscriptions.has(ev.task_id)) {
      const gap = detectGap(this.opts.getLastSeq(ev.task_id), ev.seq);
      if (gap) this.opts.onGap?.(ev.task_id, gap.from, gap.to);
    }
    this.opts.onEvent(ev);
  }

  private send(obj: object): void {
    if (this.isOpen) this.ws!.send(JSON.stringify(obj));
  }

  private startHeartbeat(): void {
    this.pingTimer = setInterval(() => {
      if (!this.isOpen) return;
      this.send({ op: "ping" });
      if (!this.pongTimer) {
        this.pongTimer = setTimeout(() => {
          // pong 없음 → 죽은 연결로 보고 끊는다 (onclose 가 재연결)
          this.ws?.close();
        }, this.opts.pongTimeoutMs);
      }
    }, this.opts.pingIntervalMs);
  }

  private scheduleReconnect(): void {
    this.attempt += 1;
    const delay = this.nextDelayMs(this.attempt);
    this.opts.onStatus?.("reconnecting", this.attempt);
    this.reconnectTimer = setTimeout(() => this.open(), delay);
  }

  private clearTimers(): void {
    if (this.reconnectTimer) clearTimeout(this.reconnectTimer);
    if (this.pingTimer) clearInterval(this.pingTimer);
    if (this.pongTimer) clearTimeout(this.pongTimer);
    this.reconnectTimer = this.pingTimer = this.pongTimer = null;
  }
}
