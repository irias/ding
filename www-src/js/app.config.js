// don't warn about "use strict"
/* jshint -W097 */
/* global app, api */
'use strict';

app.config(function($routeProvider, $uibTooltipProvider) {
	$uibTooltipProvider.options({
		placement: 'right',
		popupDelay: 500, // ms
		appendToBody: true
	});

	$routeProvider
	.when('/', {
		templateUrl: 'static/html/index.html',
		controller: 'Index',
		resolve: {
			repoBuilds: function() {
				return api.repoBuilds();
			}
		}
	})
	.when('/repo/:repoName/', {
		templateUrl: 'static/html/repo.html',
		controller: 'Repo',
		resolve: {
			repo: function($route) {
				return api.repo($route.current.params.repoName);
			},
			repo_config: function($route) {
				return api.repoConfig($route.current.params.repoName);
			},
			builds: function($route) {
				return api.builds($route.current.params.repoName);
			}
		}
	})
	.when('/repo/:repoName/build/:buildId/', {
		templateUrl: 'static/html/repo-build.html',
		controller: 'RepoBuild',
		resolve: {
			repo: function($route) {
				return api.repo($route.current.params.repoName);
			},
			buildResult: function($route) {
				return api.buildResult($route.current.params.repoName, parseInt($route.current.params.buildId));
			}
		}
	})
	.otherwise({
		templateUrl: 'static/html/404.html'
	});
});
