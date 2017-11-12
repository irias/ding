/* jshint -W097 */ // for "use strict"
/* global app, console */
'use strict';

app
.directive('buildStatus', function() {
	return {
		restrict: 'E',
		template: '<span><span class="label" ng-class="{\'label-primary\': released && status === \'success\', \'label-success\': !released && finish && status === \'success\', \'label-danger\': finish && status !== \'success\', \'label-default\': !finish}" style="margin-right: 0.25rem">{{ status }}</span><span class="fa fa-cog fa-spin" ng-if="!finish && status !== \'new\'" style="vertical-align: middle"></span></span>',
		scope: {
			'status': '=',
			'finish': '=',
			'released': '='
		}
	};
});
