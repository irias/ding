select assert_schema_version(2);
insert into schema_upgrades (version) values (3);

alter table repo add unique (name);
