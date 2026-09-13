import type { Stage } from "@/lib/api/schemas";

export type Column = Stage[];

/**
 * 12-디자인 §3.3 PipelineBar 레이아웃 — depends_on 기준 위상 정렬을 열(column) 로.
 * 같은 열의 Stage 는 세로로 쌓여 병렬을 표현한다. 사이클/미상 의존은 마지막 열에 몰아 넣는다.
 */
export function layoutStages(stages: Stage[]): Column[] {
  const byKey = new Map(stages.map((s) => [s.key, s]));
  const level = new Map<string, number>();
  const visiting = new Set<string>();

  const depth = (key: string): number => {
    const cached = level.get(key);
    if (cached !== undefined) return cached;
    if (visiting.has(key)) return 0; // 사이클 방어
    visiting.add(key);
    const s = byKey.get(key);
    const deps = (s?.depends_on ?? []).filter((d) => byKey.has(d));
    const d = deps.length ? Math.max(...deps.map(depth)) + 1 : 0;
    visiting.delete(key);
    level.set(key, d);
    return d;
  };

  for (const s of stages) depth(s.key);
  const cols: Column[] = [];
  for (const s of stages) {
    const d = level.get(s.key) ?? 0;
    (cols[d] ??= []).push(s);
  }
  return cols.filter(Boolean);
}
