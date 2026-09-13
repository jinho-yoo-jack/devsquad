"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { useCreateTask, useProjects } from "@/lib/queries/tasks";
import { Button } from "@/components/common/Button";
import { InlineError } from "@/components/common/InlineError";

const EXAMPLES = ["설정 페이지에 다크 모드 토글 추가", "회원 탈퇴 기능 추가. 탈퇴 후 30일 유예, 유예 기간 내 복구 가능", "북마크 태그 필터링"];

export default function NewTaskPage() {
  const router = useRouter();
  const projects = useProjects();
  const create = useCreateTask();
  const [projectId, setProjectId] = useState("");
  const [command, setCommand] = useState("");
  const pid = projectId || projects.data?.[0]?.id || "";

  const submit = () =>
    create.mutate({ project_id: pid, command: command.trim() }, { onSuccess: (t) => router.push(`/tasks/${t.id}`) });

  return (
    <section className="mx-auto max-w-2xl space-y-4">
      <h1 className="text-xl font-semibold">새 Task</h1>
      <label className="block text-sm">
        <span style={{ color: "var(--text-2)" }}>프로젝트</span>
        <select value={pid} onChange={(e) => setProjectId(e.target.value)} className="mt-1 w-full rounded-md border p-2"
          style={{ borderColor: "var(--border)", background: "var(--surface-1)", color: "var(--text-1)" }}>
          {projects.data?.map((p) => <option key={p.id} value={p.id}>{p.name} ({p.github_owner}/{p.github_repo})</option>)}
          {projects.data?.length === 0 && <option value="">프로젝트가 없습니다 — /projects 에서 먼저 만드세요</option>}
        </select>
      </label>
      <label className="block text-sm">
        <span style={{ color: "var(--text-2)" }}>명령</span>
        <textarea value={command} onChange={(e) => setCommand(e.target.value)} rows={5} placeholder={EXAMPLES[0]}
          className="mt-1 w-full rounded-md border p-2" style={{ borderColor: "var(--border)", background: "var(--surface-1)", color: "var(--text-1)" }} />
      </label>
      <div className="flex flex-wrap gap-1.5 text-xs">
        {EXAMPLES.map((e) => <button key={e} onClick={() => setCommand(e)} className="rounded-full border px-2 py-0.5" style={{ borderColor: "var(--border)", color: "var(--text-2)" }}>{e}</button>)}
      </div>
      {create.isError && <InlineError message={create.error instanceof Error ? create.error.message : "생성 실패"} />}
      <Button variant="primary" loading={create.isPending} disabled={!pid || command.trim().length < 3} onClick={submit}>팀에 맡기기</Button>
    </section>
  );
}
