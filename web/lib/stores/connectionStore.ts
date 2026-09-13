import { create } from "zustand";

export type ConnectionStatus = "idle" | "connecting" | "open" | "reconnecting" | "closed";

type ConnectionStore = {
  status: ConnectionStatus;
  attempt: number;
  lastError: string | null;
  set: (patch: Partial<Omit<ConnectionStore, "set">>) => void;
};

export const useConnectionStore = create<ConnectionStore>((set) => ({
  status: "idle",
  attempt: 0,
  lastError: null,
  set: (patch) => set(patch),
}));
