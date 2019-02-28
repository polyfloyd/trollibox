'use strict';

var NetModel = Backbone.Model.extend({
	updating: {},

	initialize: function(args) {
		var proto = window.location.protocol;
		var path = URLROOT.replace(/^https?:(\/\/)?/, '');
		if (path === '/') {
			path = window.location.host+path;
		}
		var url = `${window.location.protocol}//${path}data${args.eventSourcePath}`;
		this.eventSource = new EventSource(url);
	},

	callServer: function(path, method, body) {
		return promiseAjax({
			url:      URLROOT+'data'+path,
			method:   method,
			dataType: 'json',
			data:     body ? JSON.stringify(body) : null,
		}).catch(function(err) {
			this.trigger('error', method+' '+path+': '+err);
		}.bind(this));
	},

	attachServerReloader: function(event, path, handler) {
		this.reloaders = this.reloaders || {};
		var property = event.split(':')[1];
		var reload = function() {
			if (!this.updating[property]) {
				this.callServer(path, 'GET', null).then(handler.bind(this));
			}
		}.bind(this);
		if (this.eventSource) {
			this.eventSource.addEventListener(property, function() { reload(); });
		} else {
			this.on(event, reload, this);
		}
		this.reloaders[event] = reload;
		reload();
	},

	attachServerUpdater: function(property, path, getUpdateData) {
		this.updating[property] = false;
		var nextValue = undefined;

		function update(value) {
			this.updating[property] = true;
			this.callServer(path, 'POST', getUpdateData.call(this, value)).then(function(data) {
				setTimeout(function() {
					this.updating[property] = false;
					if (typeof nextValue !== 'undefined') {
						update.call(this, nextValue);
						nextValue = undefined;
					}
				}.bind(this), 200);
			}.bind(this)).catch(function() {
				this.updating[property] = false;
			});
		}

		this.on('change:'+property, function(obj, value, options) {
			if (options.sender === this) {
				return;
			}
			if (this.updating[property]) {
				nextValue = value;
			} else {
				update.call(this, value);
			}
		});
	},

	reload: function(name) {
		this.reloaders = this.reloaders || {};
		if (typeof name !== 'string') {
			for (var k in this.reloaders) {
				this.reloaders[k].call(this);
			}
		} else {
			if (this.reloaders[name]) {
				this.reloaders[name].call(this);
			}
		}
	},

	/**
	 * Like the regular Backbone.Model#set(), but propagates a flag to change
	 * listeners so they can differentiate between events fired from external
	 * (e.g. view) and internal (e.g. reload*).
	 */
	setInternal: function(key, value, options) {
		options = options || {};
		options.sender = this;
		return Backbone.Model.prototype.set.call(this, key, value, options);
	},
});
