/* jshint -W097 */ // for "use strict"
/* global app */
'use strict';

app
.directive('clickStopPropagation', function() {
	return {
		restrict: 'A',
		link: function(scope, element) {
			element.bind('click', function(e) {
				e.stopPropagation();
			});
		}
	};
});
