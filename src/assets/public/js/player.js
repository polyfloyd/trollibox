'use strict';

var Player = Backbone.Model.extend({
	initialize: function() {
		//this.on('server-event:progress', this.reloadCurrent, this);
		this.on('server-event:current', this.reloadCurrent, this);
		this.on('change:current', this.reloadProgressUpdater, this);
		this.on('change:state', this.reloadProgressUpdater, this);

		this.on('server-connect', this.reload, this);
		this.connectEventSocket();
	},

	connectEventSocket: function() {
		var self = this;

		var wsRoot = URLROOT.replace(/^http/, 'ws');
		this.sock = new WebSocket(wsRoot+'data/listen');
		this.sock.onopen = function() {
			self.sock.onerror = function() {
				self.sock.close();
			};
			self.trigger('server-connect');
		};
		this.sock.onclose = function() {
			setTimeout(function() {
				self.connectEventSocket();
			}, 1000 * 4);
		};

		this.sock.onmessage = function(event) {
			self.trigger('server-event:'+event.data);
		};
	},

	reload: function() {
		this.reloadCurrent();
	},

	reloadProgressUpdater: function() {
		var self = this;

		clearInterval(this.progressUpdater);
		clearTimeout(this.progressTimeout);

		if (this.get('current') && this.get('state') === 'playing') {
			this.progressUpdater = setInterval(function() {
				self.set('progress', self.get('progress') + 1);
			}, 1000);
			this.progressTimeout = setTimeout(function() {
				self.reloadCurrent();
			}, 1000 * (this.get('current').duration - this.get('progress')));
		}
	},

	reloadCurrent: function(cb) {
		$.ajax({
			url:      URLROOT+'data/track/current',
			method:   'GET',
			dataType: 'json',
			context:  this,
			success:  function(data) {
				this.set('current',  data.track);
				this.set('progress', data.progress);
				this.set('state',    data.state);
				if (cb) cb.call(this);
			},
			error:    function(req, str, err) {
				this.trigger('error', err);
			},
		});
	},
});
