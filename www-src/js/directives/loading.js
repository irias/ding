// don't warn about "use strict"
/* jshint -W097 */
/* global app */
'use strict';

app.directive('loadingClick', function($rootScope, Msg) {
	return {
		restrict: 'A',
		link: function(scope, element, attrs) {
			element.on('click', function(e) {
				e.preventDefault();
				e.stopPropagation();

				if (element.attr('disabled')) {
					return;
				}

				scope.$apply(function() {
					$rootScope.loading = true;
				});

				scope.$eval(attrs.loadingClick)
				.then(function() {
					$rootScope.loading = false;
				}, function(error) {
					$rootScope.loading = false;
					if(error) {
						Msg.alert(error.message);
					}
				});
			});
		}
	};
})
.directive('savingClick', function($rootScope, Msg) {
	return {
		restrict: 'A',
		link: function(scope, element, attrs) {
			element.on('click', function(e) {
				e.preventDefault();
				e.stopPropagation();

				if (element.attr('disabled')) {
					return;
				}

				scope.$apply(function() {
					$rootScope.loading = true;
				});

				scope.$eval(attrs.savingClick)
				.then(function() {
					$rootScope.loadingSaved();
				}, function(error) {
					$rootScope.loading = false;
					if(error) {
						Msg.alert(error.message);
					}
				});
			});
		}
	};
})
.directive('loadingSubmit', function($rootScope, Msg) {
	return {
		restrict: 'A',
		link: function(scope, element, attrs) {
			element.on('submit', function(e) {
				e.preventDefault();
				e.stopPropagation();

				if (element.attr('disabled')) {
					return;
				}

				scope.$apply(function() {
					$rootScope.loading = true;
				});

				scope.$eval(attrs.loadingSubmit)
				.then(function() {
					$rootScope.loading = false;
				}, function(error) {
					$rootScope.loading = false;
					if(error) {
						Msg.alert(error.message);
					}
				});
			});
		}
	};
})
.directive('savingSubmit', function($rootScope, Msg) {
	return {
		restrict: 'A',
		link: function(scope, element, attrs) {
			element.on('submit', function(e) {
				e.preventDefault();
				e.stopPropagation();

				if (element.attr('disabled')) {
					return;
				}

				scope.$apply(function() {
					$rootScope.loading = true;
				});

				scope.$eval(attrs.savingSubmit)
				.then(function() {
					$rootScope.loadingSaved();
				}, function(error) {
					$rootScope.loading = false;
					if(error) {
						Msg.alert(error.message);
					}
				});
			});
		}
	};
});
