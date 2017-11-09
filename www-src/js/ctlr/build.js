// don't warn about "use strict"
/* jshint -W097 */
/* global app, api, _, console, window */
'use strict';

app.controller('Build', function($scope, $rootScope, $q, $location, $timeout, Msg, Util, repo, buildResult) {
	$rootScope.breadcrumbs = Util.crumbs([
		Util.crumb('repo/' + repo.name, 'Repo ' + repo.name),
		Util.crumb('build/' + buildResult.build.id + '/', 'Build ' + buildResult.build.id)
	]);

	$scope.repo = repo;
	$scope.buildResult = buildResult;
	$scope.build = buildResult.build;
	$scope.steps = buildResult.steps;

	$scope.$on('build', function(x, e) {
		var b = e.build;
		if (b.id !== $scope.build.id) {
			return;
		}
		$timeout(function() {
			$scope.build = b;
		});
	});

	$scope.$on('removeBuild', function(x, e) {
		if (e.build_id === $scope.build.id) {
			$location.path('/repo/' + repo.name + '/');
			return;
		}
	});

	$scope.$on('output', function(x, e) {
		if (e.build_id !== $scope.build.id) {
			return;
		}
		$timeout(function() {
			var step = _.find($scope.steps, {name: e.step});
			if (!step) {
				step = {
					name: e.step,
					output: '',
					// nsec: 0,
					_start: new Date().getTime()
				};
				$scope.steps.push(step);
			}
			step.output += e.text;
		});
	});


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
		return api.createBuild(repo.name, build.branch, build.commit_hash)
		.then(function(nbuild) {
			$location.path('/repo/' + repo.name + '/build/' + nbuild.id + '/');
		});
	};

	$scope.release = function() {
		var build = $scope.build;
		return api.createRelease(repo.name, build.id)
		.then(function(nbuild) {
			$location.path('/repo/' + repo.name + '/release/' + build.id + '/');
		});
	};

	$scope.cleanupBuilddir = function() {
		var build = $scope.build;
		return api.cleanupBuilddir(repo.name, build.id)
		.then(function(nbuild) {
			$location.path('/repo/' + repo.name + '/');
		});
	};
});
