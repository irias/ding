select assert_schema_version(5);
insert into schema_upgrades (version) values (6);

update build set branch = 'x' where branch = '';
alter table build add constraint branch_not_empty check (branch != '');
