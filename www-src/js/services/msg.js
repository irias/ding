// don't warn about "use strict"
/* jshint -W097 */
/* global app, _ */
'use strict';

app.service('Msg', function($q, $window, $rootScope, $uibModal, $sce) {
	this.alert = function(message) {
		return $uibModal.open({
			templateUrl: 'static/html/modals/alert.html',
			controller: function($scope, $uibModalInstance) {
				$scope.title = 'Fout!';
				$scope.message = message;
				$scope.alertClass = 'danger';

				$scope.close = function() {
					$uibModalInstance.close();
				};
			}
		}).opened;
	};

	this.dialog = function(message, alertClass) {
		return $uibModal.open({
			templateUrl: 'static/html/modals/alert.html',
			controller: function($scope, $uibModalInstance) {
				$scope.title = {
					danger: 'Fout',
					warning: 'Waarschuwing',
					info: 'Geslaagd'
				}[alertClass];
				$scope.message = message;
				$scope.alertClass = alertClass;

				$scope.close = function() {
					$uibModalInstance.close();
				};
			}
		}).opened;
	};

	this.confirm = function confirm(message, handle) {
		if (!message) {
			message = 'Weet je het zeker?';
		}

		return $uibModal.open({
			templateUrl: 'static/html/modals/confirm.html',
			controller: function($scope, $uibModalInstance) {
				$scope.message = message;

				$scope.confirm = function() {
					$uibModalInstance.close();
					return handle();
				};

				$scope.dismiss = function() {
					$uibModalInstance.dismiss();
				};
			}
		}).opened;
	};

	this.linkPost = function linkPost(url, message, action, pairs) {
		$sce.trustAsUrl(url);

		return $uibModal.open({
			templateUrl: 'static/html/modals/link-post.html',
			controller: function($scope, $uibModalInstance) {
				$scope.url = url;
				$scope.message = message;
				$scope.action = action;
				$scope.pairs = pairs;
				$scope.close = function() {
					$uibModalInstance.close();
				};
			}
		}).opened;
	};

	this.link = function link(url, action, message) {
		$sce.trustAsUrl(url);

		return $uibModal.open({
			templateUrl: 'static/html/modals/link.html',
			controller: function($scope, $uibModalInstance) {
				$scope.url = url;
				$scope.message = message;
				$scope.action = action;

				$scope.close = function() {
					$uibModalInstance.close();
				};
			}
		}).opened;
	};
});
