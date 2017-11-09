// don't warn about "use strict"
/* jshint -W097 */
/* global api, $, window, angular, _, console */
'use strict';

var app = angular.module('app', [
	'templates',
	'ngRoute',
	'ui.bootstrap',
	'ui.bootstrap.modal',
	'ui.bootstrap.popover',
	'ui.bootstrap.tooltip',
	'ui.bootstrap.progressbar',
	'ui.bootstrap.tabs',
	'ui.bootstrap.datepickerPopup'
])
.run(function($rootScope, $window, $uibModal, $q, $timeout, Msg, Util) {
	api._wrapThenable = $q;

	$rootScope._app_version = api._sherpa.version;

	$rootScope.loading = false;
	$rootScope.loadingSaved = function() {
		$rootScope.loading = false;
		$('.x-loadingsaved').show().delay(1500).fadeOut('slow');
	};

	var handleApiError = function(error) {
		console.log('Error loading page', error);
		var txt;
		if(_.has(error, 'message')) {
			txt = error.message;
		} else {
			txt = JSON.stringify(error);
		}
		Msg.alert('Error loading page: ' + txt);
	};

	$rootScope.$on('$routeChangeStart', function(event, next) {
		$rootScope.loading = true;
		$rootScope.breadcrumbs = [];
	});

	$rootScope.$on('$routeChangeSuccess', function() {
		$rootScope.loading = false;
	});

	$rootScope.$on('$routeChangeError', function(event, current, previous, rejection) {
		$rootScope.loading = false;
		handleApiError(rejection);
	});

	if (!!window.EventSource) {
		var events = new window.EventSource('/events');
		events.addEventListener('message', function(e) {
			var m = JSON.parse(e.data);
			$rootScope.$broadcast(m.kind, m);
		});
		events.addEventListener('open', function(e) {
			$timeout(function() {
				$rootScope.sseError = '';
			});
		});
		events.addEventListener('error', function(e) {
			$timeout(function() {
				$rootScope.sseError = 'Connection error, no live updates.';
			});
		});
	} else {
		$rootScope.noSSE = true;
	}
});
