/* jshint -W097 */ // for "use strict"
/* global app */
'use strict';

app
.directive('linkDisabled', function() {
	return {
		restrict: 'A',
		scope: {
			'linkDisabled': '='
		},
		link: function(scope, element) {
			scope.$watch('linkDisabled', function(v) {
				if(v) {
					element.attr('disabled', 'disabled');
				} else {
					element.removeAttr('disabled');
				}
			});
			element.bind('click', function(e) {
				if(scope.linkDisabled) {
					e.stopPropagation();
					e.preventDefault();
				}
			});
		}
	};
});
