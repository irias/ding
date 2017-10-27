/* jshint -W097 */ // for "use strict"
/* global app, console */
'use strict';

app
.directive('buildtime', function() {
	return {
		restrict: 'E',
		template: '<span>{{ elapsed.toFixed(1) }}s<span ng-if="!finish">...</span></span>',
		scope: {
			'start': '=',
			'finish': '='
		},
		link: function(scope, element) {
			var finish = scope.finish ? new Date(scope.finish) : new Date();
			scope.elapsed = (finish.getTime() - new Date(scope.start).getTime()) / 1000;
		}
	};
});
