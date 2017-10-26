create table repo (
	id serial primary key,
	name text not null,
	origin text not null
);

create table build (
	id serial primary key,
	repo_id int not null references repo(id),
	branch text not null,
	commit_hash text not null,
	status text not null check(status in ('new', 'clone', 'checkout', 'build', 'test', 'release', 'success')),
	start timestamptz not null default now(),
	finish timestamptz
);

create table result (
	id serial primary key,
	build_id int not null references build(id),
	command text not null,
	version text not null,
	os text not null,
	arch text not null,
	toolchain text not null,
	filename text not null
);

create view build_with_result as
select
	build.*,
	array_remove(array_agg(result.*), null) as results
from build
left join result on build.id = result.build_id
group by build.id
;

create table schema_upgrades (
	version int not null unique,
	upgraded timestamp without time zone default (now() at time zone 'utc')
);
insert into schema_upgrades (version) values (0);


create or replace function assert_schema_version(expected_version int) returns void as $$
declare
	current_version int;
begin
	select max(version) into current_version from schema_upgrades;
	if current_version != expected_version then
		raise exception 'cannot perform schema upgrade: wrong versions, expected version %, saw version %', expected_version, current_version;
	end if;
end;
$$ language plpgsql;


-- in future upgrade scripts, assert that the version is the one expect:
select assert_schema_version(0);
-- insert into schema_upgrades (version) values (1);
