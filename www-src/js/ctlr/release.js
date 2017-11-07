// don't warn about "use strict"
/* jshint -W097 */
/* global app, api */
'use strict';

app.controller('Release', function($scope, $rootScope, $q, $location, Msg, Util, repo, buildResult) {
	$rootScope.breadcrumbs = Util.crumbs([
		Util.crumb('repo/' + repo.name, 'Repo ' + repo.name),
		Util.crumb('release/' + buildResult.build.id + '/', 'Release ' + buildResult.build.id)
	]);


	$scope.repo = repo;
	$scope.buildResult = buildResult;
	$scope.build = buildResult.build;
	$scope.steps = buildResult.steps;
});
