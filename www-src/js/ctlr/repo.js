// don't warn about "use strict"
/* jshint -W097 */
/* global app, api, _ */
'use strict';

app.controller('Repo', function($scope, $rootScope, $q, $location, Msg, Util, repo, repo_config, builds) {
	$rootScope.breadcrumbs = Util.crumbs([
		Util.crumb('repo/' + repo.name, 'Repo ' + repo.name)
	]);


	$scope.repo = repo;
	$scope.repo_config = repo_config;
	$scope.builds = builds;
	$scope.releaseBuilds = _.filter($scope.builds, function(b) { return b.released; });

	$scope.removeRepo = function() {
		return Msg.confirm('Are you sure?', function() {
			return api.removeRepo(repo.name)
			.then(function() {
				$location.path('/');
			});
		});
	};

	$scope.save = function() {
		return api.saveRepo($scope.repo, $scope.repo_config);
	};

	$scope.removeBuild = function(build) {
		return Msg.confirm('Are you sure?', function() {
			return api.removeBuild(build.id)
			.then(function() {
				$scope.builds = _.filter($scope.builds, function(b) {
					return b.id !== build.id;
				});
			});
		});
	};

	$scope.retryBuild = function(build) {
		return api.buildStart(repo.name, build.branch, build.commit_hash).
		then(function(nbuild) {
			$location.path('/repo/' + repo.name + '/build/' + nbuild.id + '/');
		});
	};

	$scope.buildBranch = function(build) {
		return api.buildStart(repo.name, build.branch, '').
		then(function(nbuild) {
			$location.path('/repo/' + repo.name + '/build/' + nbuild.id + '/');
		});
	};

	$scope.cleanupBuilddir = function(build) {
		return api.cleanupBuilddir(repo.name, build.id)
		.then(function(nbuild) {
			$scope.builds = _.map($scope.builds, function(b) {
				return b.id === build.id ? nbuild : b;
			});
		});
	};
});
