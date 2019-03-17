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
		this.eventSource.onopen = () => {
			// Reload all state to ensure that we are in sync.
			this.reload();
		};
	},

	callServer: function(path, method, body) {
		return fetch(`${URLROOT}data${path}`, {
			method: method,
			headers: {
				'Content-Type': 'application/json',
			},
			body: body ? JSON.stringify(body) : null,
		})
		.then((resp) => {
			if (resp.status >= 200 && resp.status < 300) {
				return Promise.resolve(resp.json());
			}
			return Promise.reject(new Error(resp.statusText));
		})
		.catch((err) => {
			this.trigger('error', method+' '+path+': '+err);
		});
	},

	attachServerReloader: function(event, path, handler) {
		this.reloaders = this.reloaders || {};
		var property = event.split(':')[1];
		var reload = () => {
			if (!this.updating[property]) {
				this.callServer(path, 'GET', null).then(handler.bind(this));
			}
		};
		if (this.eventSource) {
			this.eventSource.addEventListener(property, () => reload());
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
			this.callServer(path, 'POST', getUpdateData.call(this, value)).then((data) => {
				setTimeout(() => {
					this.updating[property] = false;
					if (typeof nextValue !== 'undefined') {
						update.call(this, nextValue);
						nextValue = undefined;
					}
				}, 200);
			}).catch(() => {
				this.updating[property] = false;
			});
		}

		this.on('change:'+property, (obj, value, options) => {
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
