import { z } from "zod";

export const Health = z.object({ status: z.string(), service: z.string(), ts: z.string() });
export type Health = z.infer<typeof Health>;

export async function fetchHealth(): Promise<Health> {
  const res = await fetch("/api/v1/health", { cache: "no-store" });
  if (!res.ok) throw new Error(`health ${res.status}`);
  return Health.parse(await res.json());
}
