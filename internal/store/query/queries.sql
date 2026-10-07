-- name: GetProject :one
select to_jsonb(p) from project p where id=$1;
-- name: ListProjects :many
select to_jsonb(p) from project p order by created_at,id;
-- name: CreateProject :exec
insert into project(id,name,github_owner,github_repo,default_branch,installation_id,local_path) values($1,$2,$3,$4,$5,$6,$7);
-- name: GetTask :one
select to_jsonb(t) from task t where id=$1;
-- name: LockTask :one
select to_jsonb(t) from task t where id=$1 for update;
-- name: ListTasks :many
select to_jsonb(t) from task t where (sqlc.narg(project_id)::text is null or project_id::text=sqlc.narg(project_id)::text) and (sqlc.narg(statuses)::text[] is null or status=any(sqlc.narg(statuses)::text[])) order by created_at desc,id;
-- name: CreateTask :exec
insert into task(id,project_id,command,status,created_by,pipeline,workspace,token_budget) values($1,$2,$3,'queued',$4,$5,$6,$7);
-- name: UpdateTaskStatus :execrows
update task set status=$2,version=version+1,updated_at=now() where id=$1 and version=$3;
-- name: UpdateTaskBudget :execrows
update task set token_budget=$2,version=version+1,updated_at=now() where id=$1 and version=$3;
-- name: TaskTokensUsed :one
-- Every token kind counts: cached input is still processed and billed.
select coalesce(sum(input_tokens+output_tokens+cache_read_tokens+cache_write_tokens),0)::bigint from usage_record where task_id=$1;
-- name: ListStages :many
select to_jsonb(s) from stage s where task_id=$1 order by stage_key;
-- name: CreateStage :exec
insert into stage(id,task_id,stage_key,role,depends_on,status) values($1,$2,$3,$4,$5,'pending');
-- name: ClaimStage :one
update stage set status=$3,attempt=attempt+1,version=version+1,started_at=coalesce(started_at,now()) where id=$1 and attempt=$2 and status=sqlc.arg(old_status) returning attempt;
-- name: UpdateStage :execrows
update stage set status=$3,plan=$4,plan_retry_no=$5,deliverable_ref=$6,deliverable_summary=$7,deliverable_retry_no=$8,last_feedback=$9,blocked_reason=$10,retry_count=$5::integer+$8::integer,version=version+1,completed_at=case when $3='approved' then now() else completed_at end where id=$1 and attempt=$2 and status=sqlc.arg(old_status) and version=sqlc.arg(old_version);
-- name: GetApproval :one
select to_jsonb(a) from approval a where id=$1;
-- name: ListApprovals :many
-- A terminal Task's approvals can no longer be decided, so a pending filter skips them.
select to_jsonb(a) from approval a where (sqlc.narg(task_id)::text is null or task_id::text=sqlc.narg(task_id)::text) and (sqlc.narg(status)::text is null or status=sqlc.narg(status)::text) and (sqlc.narg(status)::text is distinct from 'pending' or exists(select 1 from task t where t.id=a.task_id and t.status not in ('completed','failed','cancelled'))) order by requested_at,id;
-- name: CreateApproval :exec
insert into approval(id,task_id,stage_id,kind,retry_no,status,title,content) values($1,$2,$3,$4,$5,'pending',$6,$7);
-- name: DecideApproval :execrows
update approval set status=$2,decision_feedback=$3,edited_content=$4,decided_by=$5,decided_via='web',decided_at=now(),version=version+1 where id=$1 and status='pending' and version=$6;
-- name: CreateDeliverable :exec
insert into deliverable(id,task_id,stage_id,kind,uri,content,summary) values($1,$2,$3,'markdown',$4,$5,$6);
-- name: NextSequence :one
update task set event_seq=event_seq+1 where id=$1 returning event_seq;
-- name: LatestDeliverableID :one
select id from deliverable where stage_id=$1 order by created_at desc,id desc limit 1;
-- name: CreateEvent :exec
insert into task_event(event_id,task_id,seq,stage_key,agent,type,payload,ts) values($1,$2,$3,$4,$5,$6,$7,$8);
-- name: CreateUsage :exec
insert into usage_record(task_id,stage_key,agent,model,input_tokens,output_tokens,cache_read_tokens,cache_write_tokens,ts) values($1,$2,$3,$4,$5,$6,$7,$8,$9);
-- name: ListEvents :many
select (to_jsonb(e)-'id')::jsonb as event from task_event e where task_id=$1 and seq>$2 order by seq limit $3::integer;
