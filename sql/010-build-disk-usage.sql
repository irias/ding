select assert_schema_version(9);
insert into schema_upgrades (version) values (10);

alter table build add column disk_usage bigint;

drop view build_with_result;
create view build_with_result as
select
	build.*,
	array_remove(array_agg(result.*), null) as results
from build
left join result on build.id = result.build_id
group by build.id
;
