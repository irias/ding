// don't warn about "use strict"
/* jshint -W097 */
/* global app, $, angular, _, window */
'use strict';

app
.filter('titleize', function() {
	return function(text) {
		return _.capitalize(text);
	};
});
