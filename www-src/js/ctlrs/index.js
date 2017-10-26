// don't warn about "use strict"
/* jshint -W097 */
/* global app, api */
'use strict';

app.controller('Index', function($scope, $rootScope, $q, $uibModal, $location, Util, repoBuilds) {
	$rootScope.breadcrumbs = Util.crumbs([]);

	$scope.repoBuilds = repoBuilds;

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
