// don't warn about "use strict"
/* jshint -W097 */
/* global app, api */
'use strict';

app.controller('RepoBuild', function($scope, $rootScope, $q, $location, Msg, Util, repo, buildResult) {
	$rootScope.breadcrumbs = Util.crumbs([
		Util.crumb('repo/' + repo.name, 'Repo ' + repo.name),
		Util.crumb('build/' + buildResult.build.id + '/', 'Build ' + buildResult.build.id)
	]);


	$scope.repo = repo;
	$scope.build = buildResult.build;
	$scope.build_config = buildResult.build_config;
	$scope.steps = buildResult.steps;

	$scope.removeBuild = function() {
		return Msg.confirm('Are you sure?', function() {
			return api.removeBuild($scope.buildResult.build.id)
			.then(function() {
				$location.path('/repo/' + repo.name + '/');
			});
		});
	};
});
