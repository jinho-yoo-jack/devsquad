import type { StageStatus, TaskStatus } from "@/lib/api/schemas";

/** 12-디자인 §5 상태 색 토큰 매핑. 색만으로 구분하지 않도록 label·icon 을 함께 준다. */
export const taskStatusMeta: Record<TaskStatus, { label: string; color: string; icon: string }> = {
  queued: { label: "대기열", color: "var(--status-pending)", icon: "○" },
  running: { label: "실행 중", color: "var(--status-running)", icon: "◐" },
  waiting_approval: { label: "승인 대기", color: "var(--status-waiting)", icon: "🙋" },
  paused: { label: "일시정지", color: "var(--status-pending)", icon: "⏸" },
  blocked: { label: "차단", color: "var(--status-rejected)", icon: "⚠" },
  completed: { label: "완료", color: "var(--status-done)", icon: "●" },
  failed: { label: "실패", color: "var(--status-failed)", icon: "✕" },
  cancelled: { label: "취소", color: "var(--status-pending)", icon: "■" },
};

export const stageStatusMeta: Record<StageStatus, { label: string; color: string; icon: string }> = {
  pending: { label: "대기", color: "var(--status-pending)", icon: "○" },
  planning: { label: "계획 중", color: "var(--status-running)", icon: "◐" },
  plan_review: { label: "계획 승인 대기", color: "var(--status-waiting)", icon: "🙋" },
  executing: { label: "실행 중", color: "var(--status-running)", icon: "◐" },
  deliverable_review: { label: "결과 승인 대기", color: "var(--status-waiting)", icon: "🙋" },
  approved: { label: "완료", color: "var(--status-done)", icon: "●" },
  blocked: { label: "차단", color: "var(--status-rejected)", icon: "⚠" },
};

const roleColors: Record<string, string> = {
  planner: "var(--role-planner)",
  designer: "var(--role-designer)",
  frontend: "var(--role-frontend)",
  backend: "var(--role-backend)",
  reviewer: "var(--role-reviewer)",
  publisher: "var(--role-publisher)",
  system: "var(--text-2)",
};

/** 알려진 역할이면 토큰, 아니면 이름 해시로 결정적 색 (사용자 정의 팀원). */
export function roleColor(role: string | null | undefined): string {
  if (!role) return "var(--text-2)";
  if (roleColors[role]) return roleColors[role];
  let h = 0;
  for (const ch of role) h = (h * 31 + ch.charCodeAt(0)) >>> 0;
  return `hsl(${h % 360} 65% 62%)`;
}

export const eventIcon: Record<string, string> = {
  "agent.thinking": "💭", "agent.tool_call": "🔧", "agent.tool_result": "✅", "agent.message": "💬",
  "approval.requested": "🙋", "approval.decided": "✔", "deliverable.produced": "📄", "usage": "▫",
  "stage.started": "▶", "stage.completed": "■", "stage.blocked": "⚠",
  "run.started": "▶", "run.resumed": "▶", "run.completed": "🏁", "run.failed": "✕", "run.paused": "⏸", "run.cancelled": "■",
  "pr.opened": "🔗", "pr.review_comment": "💬",
};

export function relativeTime(iso: string, now = Date.now()): string {
  const d = (now - new Date(iso).getTime()) / 1000;
  if (d < 5) return "방금";
  if (d < 60) return `${Math.floor(d)}초 전`;
  if (d < 3600) return `${Math.floor(d / 60)}분 전`;
  if (d < 86400) return `${Math.floor(d / 3600)}시간 전`;
  return `${Math.floor(d / 86400)}일 전`;
}

export function clockTime(iso: string): string {
  const t = new Date(iso);
  return `${String(t.getHours()).padStart(2, "0")}:${String(t.getMinutes()).padStart(2, "0")}:${String(t.getSeconds()).padStart(2, "0")}`;
}
