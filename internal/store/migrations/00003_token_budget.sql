-- +goose Up
-- Each Task keeps the budget it started with so increases apply to that Task only.
alter table task add column token_budget bigint;
update task t set token_budget=p.token_budget from project p where p.id=t.project_id;
alter table task alter column token_budget set not null,
 add constraint ck_task_token_budget check (token_budget > 0);
