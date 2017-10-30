// don't warn about "use strict"
/* jshint -W097 */
/* global app */
'use strict';

app
.filter('basename', function() {
	return function(text) {
		var t = text.split('/');
		return t[t.length-1];
	};
});
