-- +goose Up
alter table task add column pipeline jsonb,
 add column workspace text,
 add column event_seq bigint not null default 0;
alter table stage add column plan text,
 add column plan_retry_no integer not null default 0,
 add column deliverable_ref text,
 add column deliverable_summary text,
 add column deliverable_retry_no integer not null default 0,
 add column last_feedback text,
 add column blocked_reason text,
 add column attempt integer not null default 0,
 add column lease_owner text,
 add column lease_until timestamptz,
 add column version integer not null default 0;
alter table task add constraint ck_task_status check (status in ('queued','running','waiting_approval','paused','blocked','completed','failed','cancelled'));
alter table stage add constraint ck_stage_status check (status in ('pending','planning','plan_review','executing','deliverable_review','approved','blocked'));
create unique index ux_approval_stage_pending on approval(stage_id) where status='pending';
create unique index ux_approval_stage_kind_retry on approval(stage_id,kind,retry_no);
alter table usage_record add column cache_read_tokens bigint not null default 0,
 add column cache_write_tokens bigint not null default 0;
