select assert_schema_version(4);
insert into schema_upgrades (version) values (5);

-- note: this script also has accomanying code that merges shell scripts from disk into this new field.
alter table repo add column build_script text default '' not null;

alter table release add column build_script text default '' not null;
update release set build_script=(build_config->>'build_script') || E'\n\necho step:test\n' || (build_config->>'test_script') || E'\n\necho step:release\n' ||  (build_config->>'release_script');
alter table release drop column build_config;
