// don't warn about "use strict"
/* jshint -W097 */
/* global app, api, _, console */
'use strict';

app.controller('Index', function($scope, $rootScope, $q, $uibModal, $location, $timeout, Util, repoBuilds) {
	$rootScope.breadcrumbs = Util.crumbs([]);

	$scope.repoBuilds = repoBuilds;

	$scope.$on('repo', function(x, e) {
		$timeout(function() {
			var r = e.repo;
			var rr = _.find($scope.repoBuilds, function(rb) {
				return rb.repo.name === r.name;
			});
			if (rr) {
				rr.repo = r;
				return;
			}
			$scope.repoBuilds.push({
				repo: r,
				builds: []
			});
		});
	});

	$scope.$on('removeRepo', function(x, e) {
		$timeout(function() {
			$scope.repoBuilds = _.filter($scope.repoBuilds, function(rb) {
				return rb.repo.name !== e.repo_name;
			});
		});
	});

	$scope.$on('build', function(x, e) {
		var b = e.build;
		var repoName = e.repo_name;
		$timeout(function() {
			var rb = _.find($scope.repoBuilds, function(rb) {
				return rb.repo.name === repoName;
			});
			if (!rb) {
				console.log('build for unknown repo?', b, repoName);
				return;
			}
			for (var i = 0; i < rb.builds.length; i++) {
				var bb = rb.builds[i];
				if (bb.id === b.id || bb.branch === b.branch) {
					rb.builds[i] = b;
					return;
				}
			}
			rb.builds.push(b);
		});
	});

	$scope.$on('removeBuild', function(x, e) {
		var build_id = e.build_id;
		$timeout(function() {
			// bug: when the most recent build is removed, this causes us to claim there are no builds (for the branch).
			for (var i = 0; i < $scope.repoBuilds.length; i++) {
				var rb = $scope.repoBuilds[i];
				rb.builds = _.filter(rb.builds, function(b) {  // jshint ignore:line
					return b.id !== build_id;
				});
			}
		});
	});


	$scope.youngestBuild = function(rb) {
		var tm;
		for(var i = 0; i < rb.builds.length; i++) {
			var b = rb.builds[i];
			if (!tm || b.start > tm) {
				tm = b.start;
			}
		}
		if (tm) {
			return new Date().getTime() - new Date(tm).getTime();
		}
		return Infinity;
	};

	$scope.newRepo = function() {
		return $uibModal.open({
			templateUrl: 'static/html/modals/new-repo.html',
			controller: function($scope, $uibModalInstance) {
				$scope.repo = {
					origin: '',
					name: ''
				};
				$scope.$watch('repo.origin', function(v) {
					if (!v || $scope.nameHadFocus) {
						return;
					}
					var repoName = _.last(v.trim('/').split(/[:\/]/)).replace(/\.git$/, '');
					$scope.repo.name = repoName;
				});

				$scope.create = function() {
					return api.createRepo($scope.repo)
					.then(function(repo) {
						$uibModalInstance.close();
						$location.path('/repo/' + repo.name);
					});
				};
				$scope.close = function() {
					$uibModalInstance.close();
				};
			}
		}).opened;
	};
});
