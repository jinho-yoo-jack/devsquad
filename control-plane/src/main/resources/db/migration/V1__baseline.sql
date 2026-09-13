-- 14-Backend 설계 A.3 스키마는 Phase 1 CP-1 에서 채운다. 골격 단계는 Flyway 파이프라인만 검증한다.
create table if not exists devsquad_meta (
  key   text primary key,
  value text not null
);
insert into devsquad_meta(key, value) values ('schema_version', '0') on conflict (key) do nothing;
