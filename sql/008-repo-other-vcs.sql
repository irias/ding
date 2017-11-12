select assert_schema_version(7);
insert into schema_upgrades (version) values (8);

alter table repo add column vcs text;
update repo set vcs = 'git';
alter table repo add constraint repo_vcs_valid check(vcs in ('git', 'mercurial', 'command'));
alter table repo alter column vcs set not null;
