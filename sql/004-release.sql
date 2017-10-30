select assert_schema_version(3);
insert into schema_upgrades (version) values (4);

alter table build add column builddir_removed boolean not null default false;
alter table build add column released timestamptz;

-- must drop & create to get new column in view
drop view build_with_result;
create view build_with_result as
select
	build.*,
	array_remove(array_agg(result.*), null) as results
from build
left join result on build.id = result.build_id
group by build.id
;

create table release (
	build_id int not null unique references build(id),
	time timestamptz not null default now(),
	build_config json not null,
	steps json not null
);
