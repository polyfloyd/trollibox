'use strict';

var Player = Backbone.Model.extend({
	initialize: function() {
		this.on('change:state', function(obj, volume, options) {
			if (options.sender === this) {
				return;
			}
			$.ajax({
				url:      URLROOT+'data/player/state',
				method:   'POST',
				dataType: 'json',
				data:     JSON.stringify({
					state: this.get('state'),
				}),
				context:  this,
				error:    function(req, str, err) {
					this.trigger('error', err);
				},
			});
		});

		this.on('server-event:player', this.reloadCurrent, this);
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

	reload: function() {
		this.reloadCurrent();
	},

	reloadProgressUpdater: function() {
		var self = this;

		clearInterval(this.progressUpdater);
		clearTimeout(this.progressTimeout);

		if (this.get('current') && this.get('state') === 'playing') {
			this.progressUpdater = setInterval(function() {
				self.setInternal('progress', self.get('progress') + 1);
			}, 1000);
			this.progressTimeout = setTimeout(function() {
				self.reloadCurrent();
			}, 1000 * (this.get('current').duration - this.get('progress')));
		}
	},

	reloadCurrent: function() {
		$.ajax({
			url:      URLROOT+'data/track/current',
			method:   'GET',
			dataType: 'json',
			context:  this,
			success:  function(data) {
				this.setInternal('current',  this.fillMissingTrackFields(data.track));
				this.setInternal('progress', data.progress);
				this.setInternal('state',    data.state);
			},
			error:    function(req, str, err) {
				this.trigger('error', err);
			},
		});
	},

	next: function() {
		$.ajax({
			url:      URLROOT+'data/player/next',
			method:   'POST',
			dataType: 'json',
			context:  this,
			error:    function(req, str, err) {
				this.trigger('error', err);
			},
		});
	},

	fillMissingTrackFields: function(track) {
		if (!track.title || !track.artist) {
			var artistAndTitle = track.id.match(/.*\/(.+)\s+-\s+(.+)\.\w+/);
			if (artistAndTitle) {
				track.artist = track.artist || artistAndTitle[1];
				track.title  = track.title  || artistAndTitle[2];
			} else {
				var title = track.id.match(/.*\/(.+)\.\w+/);
				track.title = title ? title[1] : '';
			}
		}
		return track;
	},
});
