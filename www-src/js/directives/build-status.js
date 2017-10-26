/* jshint -W097 */ // for "use strict"
/* global app, console */
'use strict';

app
.directive('buildStatus', function() {
	return {
		restrict: 'E',
		template: '<span class="label" ng-class="{\'label-success\': status === \'success\', \'label-danger\': status !== \'success\'}">{{ status }}</span>',
		scope: {
			'status': '='
		}
	};
});
