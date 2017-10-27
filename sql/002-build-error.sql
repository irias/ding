select assert_schema_version(1);
insert into schema_upgrades (version) values (2);

alter table build add column error_message text not null default '';

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
