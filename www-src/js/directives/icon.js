/* jshint -W097 */ // for "use strict"
/* global app, $, document */
'use strict';

app
.directive('icon', function() {
	return {
		priority: 20,
		link: function(scope, element, attrs) {
			var $icon = $('<i class="fa"></i>');
			$icon.addClass('fa-'+attrs.icon);
			var $a = element.find('a');
			if($a.length) {
				element = $a;
			}
			element.prepend($(document.createTextNode(' ')));
			element.prepend($icon);
		}
	};
});
