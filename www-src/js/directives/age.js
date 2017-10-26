/* jshint -W097 */ // for "use strict"
/* global app, console */
'use strict';

app
.directive('age', function() {
	return {
		restrict: 'E',
		template: '{{ age }}',
		scope: {
			'time': '='
		},
		link: function(scope, element) {
			var sec = parseInt(new Date().getTime() - new Date(scope.time).getTime()) / 1000;
			var age;
			if (sec < 60) {
				age = 'now';
			} else if (sec < 120*60) {
				age = Math.round(sec / 60) + ' mins';
			} else if (sec < 48*3600) {
				age = Math.round(sec / 3600) + ' hours';
			} else if (sec < 21*24*3600) {
				age = Math.round(sec / (24*3600)) + ' days';
			} else {
				age = Math.round(sec / (7*24*3600)) + ' weeks';
			}
			scope.age = age;
		}
	};
});
