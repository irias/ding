select assert_schema_version(0);
insert into schema_upgrades (version) values (1);

alter table result add column filesize bigint not null default 0;
