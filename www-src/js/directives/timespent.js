/* jshint -W097 */ // for "use strict"
/* global app, console */
'use strict';

app
.directive('timespent', function() {
	return {
		restrict: 'E',
		template: '{{ (nsec / (1000 * 1000)).toFixed(0) }} ms',
		scope: {
			'nsec': '='
		}
	};
});
