/* jshint -W097 */ // for "use strict"
/* global app */
'use strict';

app
.directive('enter', function() {
	return {
		restrict: 'A',
		link: function(scope, element, attrs) {
			element.bind('keydown keypress', function(e) {
				var key = 'which' in e ? e.which : e.keyCode;
				if(key === 13) {
					// enter
					scope.$apply(function() {
						scope.$eval(attrs.enter);
					});
					e.preventDefault();
				}
			});
		}
	};
});
