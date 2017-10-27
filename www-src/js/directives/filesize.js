/* jshint -W097 */ // for "use strict"
/* global app, console */
'use strict';

app
.directive('filesize', function() {
	return {
		restrict: 'E',
		template: '{{ (size / (1024*1024)).toFixed(1) }}mb',
		scope: {
			'size': '='
		}
	};
});
