/* jshint -W097 */ // for "use strict"
/* global app, location */
'use strict';

app
.directive('rowClick', function() {
	return {
		restrict: 'A',
		link: function(scope, element, attrs) {
			element.addClass('clickrow');
			element.on('click', function(e) {
				e.preventDefault();
				e.stopPropagation();

				location.href = attrs.rowClick;
			});
		}
	};
});
