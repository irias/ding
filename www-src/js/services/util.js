// don't warn about "use strict"
/* jshint -W097 */
/* global app, api, console, window */
'use strict';

app.factory('Util', function($q, $window, $rootScope, $uibModal) {

	function readFile($file) {
		if($file.length !== 1) {
			return $q.reject({message: 'Bad input type=file'});
		}
		var files = $file[0].files;
		if(files.length != 1) {
			return $q.reject({message: 'Need exactly 1 file.'});
		}
		var file = files[0];

		var defer = $q.defer();
		var fr = new window.FileReader();
		fr.onload = function(e) {
			defer.resolve(e.target.result);
		};
		fr.onerror = function(e) {
			console.log('error', e);
			defer.reject({message: 'Error reading file'});
		};
		fr.readAsDataURL(file);
		return defer.promise;
	}

	function crumb(path, label) {
		return {path: path, label: label};
	}

	function crumbs(l) {
		for(var i = 1; i < l.length; i++) {
			l[i].path = l[i-1].path+'/'+l[i].path;
		}
		return l;
	}

	return {
		readFile: readFile,
		crumb: crumb,
		crumbs: crumbs
	};
});
