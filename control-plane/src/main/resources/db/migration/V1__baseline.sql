-- 14-Backend 설계 A.3 — Control Plane 스키마 (public).
-- LangGraph 체크포인트는 Agent Runtime 이 langgraph 스키마에 별도로 만든다.

create table project (
  id               uuid primary key,
  name             text not null,
  github_owner     text not null,
  github_repo      text not null,
  default_branch   text not null default 'main',
  installation_id  bigint,
  context_path     text not null default '.devsquad',
  token_budget     bigint not null default 2000000,
  created_at       timestamptz not null default now()
);

create table task (
  id                 uuid primary key,
  project_id         uuid not null references project(id),
  command            text not null,
  status             text not null,   -- queued|running|waiting_approval|paused|blocked|completed|failed|cancelled
  discord_thread_id  text,
  created_by         text not null,
  created_at         timestamptz not null default now(),
  updated_at         timestamptz not null default now(),
  version            integer not null default 0
);
create index ix_task_project_status on task(project_id, status);

create table stage (
  id            uuid primary key,
  task_id       uuid not null references task(id),
  stage_key     text not null,
  role          text not null,
  depends_on    text[] not null default '{}',
  status        text not null,   -- pending|planning|plan_review|executing|deliverable_review|approved|blocked
  retry_count   integer not null default 0,
  started_at    timestamptz,
  completed_at  timestamptz,
  constraint ux_stage_task_key unique (task_id, stage_key)
);

create table approval (
  id                 uuid primary key,
  task_id            uuid not null references task(id),
  stage_id           uuid not null references stage(id),
  kind               text not null,   -- plan|deliverable
  retry_no           integer not null default 0,
  status             text not null,   -- pending|approved|rejected|edited
  title              text not null,
  content            text not null,
  decision_feedback  text,
  edited_content     text,
  decided_by         text,
  decided_via        text,            -- web|discord
  resume_pending     boolean not null default false,
  requested_at       timestamptz not null default now(),
  decided_at         timestamptz,
  version            integer not null default 0
);
create index ix_approval_task_status on approval(task_id, status);
create index ix_approval_pending on approval(status) where status = 'pending';

create table deliverable (
  id          uuid primary key,
  task_id     uuid not null references task(id),
  stage_id    uuid not null references stage(id),
  kind        text not null,   -- markdown|json|diff|pr
  uri         text,
  content     text,
  commit_sha  text,
  summary     text,
  created_at  timestamptz not null default now()
);
create index ix_deliverable_task on deliverable(task_id);

create table task_event (
  id         bigserial primary key,
  event_id   uuid not null,
  task_id    uuid not null references task(id),
  seq        bigint not null,
  stage_key  text,
  agent      text,
  type       text not null,
  payload    jsonb not null,
  ts         timestamptz not null,
  constraint ux_task_event_event_id unique (event_id),
  constraint ux_task_event_task_seq unique (task_id, seq)
);
create index ix_task_event_task_seq on task_event(task_id, seq);

create table usage_record (
  id             bigserial primary key,
  task_id        uuid not null references task(id),
  stage_key      text,
  agent          text,
  model          text not null,
  input_tokens   bigint not null,
  output_tokens  bigint not null,
  ts             timestamptz not null default now()
);
create index ix_usage_task on usage_record(task_id);
