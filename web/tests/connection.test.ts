import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { WsConnection } from "@/lib/ws/connection";

/** 최소 mock WebSocket. 테스트가 open()/receive()/fail() 로 서버를 흉내 낸다. */
class MockSocket {
  static instances: MockSocket[] = [];
  readyState = 0;
  sent: string[] = [];
  onopen: ((ev: unknown) => void) | null = null;
  onmessage: ((ev: { data: string }) => void) | null = null;
  onclose: ((ev: unknown) => void) | null = null;
  onerror: ((ev: unknown) => void) | null = null;
  constructor(public url: string) {
    MockSocket.instances.push(this);
  }
  send(d: string) {
    this.sent.push(d);
  }
  close() {
    if (this.readyState === 3) return;
    this.readyState = 3;
    this.onclose?.({});
  }
  // --- 서버 흉내 ---
  open() {
    this.readyState = 1;
    this.onopen?.({});
  }
  receive(obj: unknown) {
    this.onmessage?.({ data: JSON.stringify(obj) });
  }
  fail() {
    this.onerror?.({});
    this.close();
  }
  sentJson(): Record<string, unknown>[] {
    return this.sent.map((s) => JSON.parse(s));
  }
}

const event = (seq: number, taskId = "t1") => ({
  event_id: `e${seq}`, task_id: taskId, seq, ts: "2026-09-13T00:00:00Z", type: "agent.message", payload: { text: "x" },
});

function make(overrides: Partial<ConstructorParameters<typeof WsConnection>[0]> = {}) {
  const lastSeq: Record<string, number> = {};
  const events: unknown[] = [];
  const gaps: [string, number, number][] = [];
  const statuses: string[] = [];
  const conn = new WsConnection({
    url: "ws://test/ws",
    getLastSeq: (id) => lastSeq[id] ?? 0,
    onEvent: (ev) => {
      events.push(ev);
      lastSeq[ev.task_id] = Math.max(lastSeq[ev.task_id] ?? 0, ev.seq);
    },
    onGap: (id, from, to) => gaps.push([id, from, to]),
    onStatus: (s) => statuses.push(s),
    wsFactory: (u) => new MockSocket(u) as unknown as WebSocket,
    baseDelayMs: 100,
    maxDelayMs: 1000,
    pingIntervalMs: 1000,
    pongTimeoutMs: 500,
    random: () => 0.5, // jitter 0
    ...overrides,
  });
  return { conn, lastSeq, events, gaps, statuses };
}

describe("WsConnection", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    MockSocket.instances = [];
  });
  afterEach(() => vi.useRealTimers());

  it("subscribes on open and delivers parsed events", () => {
    const { conn, events, statuses } = make();
    conn.subscribe("t1");
    conn.connect();
    const ws = MockSocket.instances[0];
    ws.open();
    expect(ws.sentJson()).toEqual([{ op: "subscribe", task_id: "t1" }]);
    ws.receive(event(1));
    ws.receive({ op: "pong" });
    ws.receive({ garbage: true }); // 무시
    expect(events).toHaveLength(1);
    expect(statuses).toEqual(["connecting", "open"]);
  });

  it("reconnects with backoff and resubscribes from lastSeq", () => {
    const { conn, statuses } = make();
    conn.subscribe("t1");
    conn.connect();
    const ws1 = MockSocket.instances[0];
    ws1.open();
    ws1.receive(event(1));
    ws1.receive(event(2));
    ws1.fail(); // 연결 끊김
    expect(statuses.at(-1)).toBe("reconnecting");
    expect(MockSocket.instances).toHaveLength(1);
    vi.advanceTimersByTime(100); // attempt 1 → baseDelay
    expect(MockSocket.instances).toHaveLength(2);
    const ws2 = MockSocket.instances[1];
    ws2.open();
    expect(ws2.sentJson()[0]).toEqual({ op: "subscribe", task_id: "t1", from_seq: 2 });
  });

  it("backoff grows exponentially and caps", () => {
    const { conn } = make();
    expect(conn.nextDelayMs(1)).toBe(100);
    expect(conn.nextDelayMs(2)).toBe(200);
    expect(conn.nextDelayMs(3)).toBe(400);
    expect(conn.nextDelayMs(10)).toBe(1000); // cap
  });

  it("reports a gap when seq jumps for a subscribed task", () => {
    const { conn, gaps } = make();
    conn.subscribe("t1");
    conn.connect();
    const ws = MockSocket.instances[0];
    ws.open();
    ws.receive(event(1));
    ws.receive(event(5));
    expect(gaps).toEqual([["t1", 1, 5]]);
  });

  it("closes when no pong arrives, then reconnects", () => {
    const { conn } = make();
    conn.connect();
    const ws = MockSocket.instances[0];
    ws.open();
    vi.advanceTimersByTime(1000); // ping
    expect(ws.sentJson().some((m) => m.op === "ping")).toBe(true);
    vi.advanceTimersByTime(500); // pong timeout → close
    expect(ws.readyState).toBe(3);
    vi.advanceTimersByTime(100); // reconnect
    expect(MockSocket.instances).toHaveLength(2);
  });

  it("does not reconnect after close()", () => {
    const { conn, statuses } = make();
    conn.connect();
    MockSocket.instances[0].open();
    conn.close();
    vi.advanceTimersByTime(5000);
    expect(MockSocket.instances).toHaveLength(1);
    expect(statuses.at(-1)).toBe("closed");
  });

  it("forwards server error ops", () => {
    const errors: string[] = [];
    const { conn } = make({ onServerError: (code) => errors.push(code) });
    conn.connect();
    const ws = MockSocket.instances[0];
    ws.open();
    ws.receive({ op: "error", code: "TASK_FORBIDDEN" });
    expect(errors).toEqual(["TASK_FORBIDDEN"]);
  });
});
