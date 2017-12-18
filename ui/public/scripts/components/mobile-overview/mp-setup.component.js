'use strict';

/**
 * @ngdoc component
 * @name mcp.component:mp-setup
 * @description
 * # mp-setup
 */
angular.module('mobileControlPanelApp').component('mpSetup', {
  template: `<div class="blank-slate-pf" ng-if="!$ctrl.hasMcpServer">
              <div class="blank-slate-pf-icon">
                <span class="pficon pficon pficon-add-circle-o"></span>
              </div>
              <h1>Get Started with the Mobile Control Panel</h1>
              <p>The Mobile Control Panel helps you to create Mobile Apps & Services.</p>
              <p>To get started, provision an instance of the Mobile Control Panel in your Project.</p>
              <p>Learn more about Mobile Apps & Services <a href="http://aerogear.org/docs/">in the documentation</a>.</p>
              <div class="blank-slate-pf-main-action">
                <a ng-href="/" class="btn btn-primary btn-lg">Provision Mobile Control Panel</a>
              </div>
            </div>`,
  bindings: {
    hasMcpServer: '<'
  }
});
