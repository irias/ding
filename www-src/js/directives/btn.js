/* jshint -W097 */ // for "use strict"
/* global app */
'use strict';

app
.directive('btn', function() {
	return {
		priority: 10,
		link: function(scope, element, attrs) {
			var $a = element.find('a');
			if($a.length) {
				element = $a;
			}
			element.addClass('btn');
			if(attrs.btn) {
				var l = attrs.btn.split(' ');
				for (var i = 0; i < l.length; i++) {
					element.addClass('btn-'+l[i]);
				}
			}
		}
	};
});
