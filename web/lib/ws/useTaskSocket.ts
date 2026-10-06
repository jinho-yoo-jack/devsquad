"use client";

import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef } from "react";
import { syncTaskEvents } from "@/lib/queries/eventSync";
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
        onGap: async (id, from, to) => {
          try {
            await syncTaskEvents(id, from, to);
          } catch {
            // The query exposes replay errors/retry UI and repairs any unresolved gap.
            void qc.invalidateQueries({ queryKey: ["events", id] });
          }
        },
        onStatus: (status, attempt) => {
          useConnectionStore.getState().set({ status, attempt, lastError: null });
          if (status === "open") {
            void qc.invalidateQueries({ queryKey: ["task"] });
            void qc.invalidateQueries({ queryKey: ["approvals"] });
            void qc.invalidateQueries({ queryKey: ["events"] });
          }
        },
        onServerError: (code) => useConnectionStore.getState().set({ lastError: code }),
      });
      shared.connect();
    }
    if (taskId) shared.subscribe(taskId, useEventStore.getState().byTask[taskId]?.lastSeq);
    return () => { if (taskId) shared?.unsubscribe(taskId); };
  }, [taskId]);
}
