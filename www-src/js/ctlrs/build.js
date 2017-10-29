// don't warn about "use strict"
/* jshint -W097 */
/* global app, api */
'use strict';

app.controller('Build', function($scope, $rootScope, $q, $location, Msg, Util, repo, buildResult) {
	$rootScope.breadcrumbs = Util.crumbs([
		Util.crumb('repo/' + repo.name, 'Repo ' + repo.name),
		Util.crumb('build/' + buildResult.build.id + '/', 'Build ' + buildResult.build.id)
	]);


	$scope.repo = repo;
	$scope.build = buildResult.build;
	$scope.build_config = buildResult.build_config;
	$scope.steps = buildResult.steps;

	$scope.removeBuild = function() {
		var build = $scope.build;
		return Msg.confirm('Are you sure?', function() {
			return api.removeBuild(build.id)
			.then(function() {
				$location.path('/repo/' + repo.name + '/');
			});
		});
	};

	$scope.retryBuild = function() {
		var build = $scope.build;
		return api.buildStart(repo.name, build.branch, build.commit_hash).
		then(function(nbuild) {
			$location.path('/repo/' + repo.name + '/build/' + nbuild.id + '/');
		});
	};

	$scope.buildBranch = function() {
		var build = $scope.build;
		return api.buildStart(repo.name, build.branch, '').
		then(function(nbuild) {
			$location.path('/repo/' + repo.name + '/build/' + nbuild.id + '/');
		});
	};
});
