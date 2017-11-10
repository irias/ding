select assert_schema_version(6);
insert into schema_upgrades (version) values (7);

alter table repo add column checkout_path text;
update repo set checkout_path = name;
alter table repo add constraint checkout_path_not_empty check(checkout_path != '');
alter table repo alter column checkout_path set not null;

