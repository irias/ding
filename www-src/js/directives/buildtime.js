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
			scope.$watch('finish', function(v) {
				var finish = v ? new Date(v) : new Date();
				scope.elapsed = (finish.getTime() - new Date(scope.start).getTime()) / 1000;
			});
		}
	};
});
