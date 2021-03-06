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
			builds: function($route) {
				return api.builds($route.current.params.repoName);
			}
		}
	})
	.when('/repo/:repoName/build/:buildId/', {
		templateUrl: 'static/html/build.html',
		controller: 'Build',
		resolve: {
			repo: function($route) {
				return api.repo($route.current.params.repoName);
			},
			buildResult: function($route) {
				return api.buildResult($route.current.params.repoName, parseInt($route.current.params.buildId));
			}
		}
	})
	.when('/repo/:repoName/release/:buildId/', {
		templateUrl: 'static/html/release.html',
		controller: 'Release',
		resolve: {
			repo: function($route) {
				return api.repo($route.current.params.repoName);
			},
			buildResult: function($route) {
				return api.release($route.current.params.repoName, parseInt($route.current.params.buildId));
			}
		}
	})
	.when('/help/', {
		templateUrl: 'static/html/help.html',
		controller: function($rootScope, Util) {
			$rootScope.breadcrumbs = Util.crumbs([
				Util.crumb('/help/', 'Help')
			]);
		}
	})
	.otherwise({
		templateUrl: 'static/html/404.html'
	});
});
