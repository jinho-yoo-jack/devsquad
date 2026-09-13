"use client";

import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef } from "react";
import { fetchEvents } from "@/lib/api/tasks";
import { useConnectionStore } from "@/lib/stores/connectionStore";
import { useEventStore } from "@/lib/stores/eventStore";
import { WsConnection } from "@/lib/ws/connection";
import { makeEventHandler } from "@/lib/ws/handlers";

let shared: WsConnection | null = null;

function wsUrl(): string {
  const proto = location.protocol === "https:" ? "wss" : "ws";
  return `${proto}://${location.host}/ws`;
}

/** 페이지 단위로 task 를 구독한다. 연결은 앱 전체에서 하나를 공유. */
export function useTaskSocket(taskId: string | null) {
  const qc = useQueryClient();
  const handlerRef = useRef(makeEventHandler(qc));

  useEffect(() => {
    if (!shared) {
      shared = new WsConnection({
        url: wsUrl(),
        getLastSeq: (id) => useEventStore.getState().byTask[id]?.lastSeq ?? 0,
        onEvent: (ev) => handlerRef.current(ev),
        onGap: async (id, from) => {
          const page = await fetchEvents(id, from, 500);
          useEventStore.getState().appendMany(id, page.items);
        },
        onStatus: (status, attempt) => useConnectionStore.getState().set({ status, attempt }),
        onServerError: (code) => useConnectionStore.getState().set({ lastError: code }),
      });
      shared.connect();
    }
    if (taskId) shared.subscribe(taskId, useEventStore.getState().byTask[taskId]?.lastSeq);
    return () => { if (taskId) shared?.unsubscribe(taskId); };
  }, [taskId]);
}
