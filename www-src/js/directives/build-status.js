/* jshint -W097 */ // for "use strict"
/* global app, console */
'use strict';

app
.directive('buildStatus', function() {
	return {
		restrict: 'E',
		template: '<span class="label" ng-class="{\'label-primary\': released && status === \'success\', \'label-success\': !released && finish && status === \'success\', \'label-danger\': finish && status !== \'success\', \'label-default\': !finish}">{{ status }}</span>',
		scope: {
			'status': '=',
			'finish': '=',
			'released': '='
		}
	};
});
