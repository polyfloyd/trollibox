'use strict';

var FilterDB = NetModel.extend({
	defaults: {
		'filters': {},
	},

	initialize: function(args) {
		var self = this;
		this.attachServerReloader('server-event:update', '/filters/', function(data) {
			var filters = {};
			function next(index) {
				if (index == Object.keys(data.filters).length) {
					self.setInternal('filters', filters);
					return;
				}
				var name = data.filters[index];
				self.callServer('/filters/'+name+'/', 'GET', null).then(function(filterData) {
					filters[name] = filterData.filter;
					next(index + 1);
				});
			}
			next(0);
		});
		NetModel.prototype.initialize.call(this, {
			eventSocketPath: '/filters/listen',
		});
	},

	store: function(name, filter) {
		this.callServer('/filters/'+name+'/', 'PUT', {
			filter: filter,
		});
//		this.filters.trigger('change:filters', this.model, this.rules);
	},

	remove: function(name) {
		this.callServer('/filters/'+name+'/', 'DELETE');
//		this.filters.trigger('change:filters', this.model, this.rules);
	},
});
