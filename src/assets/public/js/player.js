'use strict';

var Player = Backbone.Model.extend({
	initialize: function() {
		this.on('change:progress', function(obj, progress, options) {
			if (options.sender === this) {
				return;
			}
			$.ajax({
				url:      URLROOT+'data/player/progress',
				method:   'POST',
				dataType: 'json',
				data:     JSON.stringify({
					progress: progress,
				}),
				context:  this,
				error:    function() {
					this.trigger('error');
				},
			});
		});
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
		this.on('change:volume', function(obj, volume, options) {
			if (options.sender === this) {
				return;
			}
			$.ajax({
				url:      URLROOT+'data/player/volume',
				method:   'POST',
				dataType: 'json',
				data:     JSON.stringify({
					volume: volume,
				}),
				context:  this,
				error:    function() {
					this.trigger('error');
				},
			});
		});
		this.on('change:playlist', function(obj, playlist, options) {
			if (options.sender === this) {
				return;
			}
			$.ajax({
				url:      URLROOT+'data/player/playlist',
				method:   'POST',
				dataType: 'json',
				data:     JSON.stringify({
					'track-ids': this.get('playlist').map(function(track) {
						return track.id;
					}),
				}),
				context:  this,
				error:    function() {
					this.trigger('error');
				},
			});
		});

		this.on('server-event:mixer',    this.reloadVolume,   this);
		this.on('server-event:player',   this.reloadCurrent,  this);
		this.on('server-event:playlist', this.reloadPlaylist, this);
		this.on('server-event:update',   this.reloadTracks,   this);
		this.on('change:current',        this.reloadProgressUpdater, this);
		this.on('change:state',          this.reloadProgressUpdater, this);

		this.on('server-connect', this.reload, this);
		this.connectEventSocket();
	},

	connectEventSocket: function() {
		var self = this;

		var wsRoot = URLROOT.replace(/^http/, 'ws');
		var sock = new WebSocket(wsRoot+'data/listen');
		sock.onopen = function() {
			self.sock = sock;
			self.sock.onerror = function() {
				self.sock.close();
			};
			self.trigger('server-connect');
		};
		sock.onclose = function() {
			if (self.sock) {
				self.trigger('error', new Error('Socket connection lost'));
			}
			self.sock = null;
			setTimeout(function() {
				self.connectEventSocket();
			}, 1000 * 4);
		};

		sock.onmessage = function(event) {
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
		this.reloadPlaylist();
		this.reloadTracks();
		this.reloadVolume();
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
			url:      URLROOT+'data/player/current',
			method:   'GET',
			dataType: 'json',
			context:  this,
			success:  function(data) {
				this.setInternal('current',  data.track ? this.fillMissingTrackFields(data.track) : null);
				this.setInternal('progress', data.progress);
				this.setInternal('state',    data.state);
			},
			error:    function(req, str, err) {
				this.trigger('error', err);
			},
		});
	},

	reloadPlaylist: function() {
		$.ajax({
			url:      URLROOT+'data/player/playlist',
			method:   'GET',
			dataType: 'json',
			context:  this,
			success:  function(data) {
				this.setInternal('playlist', data.tracks.filter(function(track) {
					return !!track.id;
				}).map(function(track) {
					return this.fillMissingTrackFields(track);
				}, this));
			},
			error:    function(req, str, err) {
				this.trigger('error', err);
			},
		});
	},

	reloadTracks: function() {
		$.ajax({
			url:      URLROOT+'data/track/browse/',
			method:   'GET',
			dataType: 'json',
			context:  this,
			success:  function(data) {
				this.setInternal('tracks', data.tracks);
			},
			error:    function(req, str, err) {
				this.trigger('error', err);
			},
		});
	},

	reloadVolume: function() {
		$.ajax({
			url:      URLROOT+'data/player/volume',
			method:   'GET',
			dataType: 'json',
			context:  this,
			success:  function(data) {
				this.setInternal('volume', data.volume);
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

	/**
	 * Search the entire library for tracks matching a word in the query string.
	 */
	search: function(query) {
		if (!query) {
			return [];
		}

		var keywords = query.toLowerCase().split(/\s+/g).filter(function(keyword) {
			return !!keyword;
		});
		if (!keywords.length) {
			return [];
		}

		return this.get('tracks').filter(function(track) {
			return keywords.every(function(keyword) {
				return ['artist', 'title', 'album'].some(function(attr) {
					var val = track[attr];
					if (typeof val === 'undefined') {
						return false;
					}
					return val.toLowerCase().indexOf(keyword) !== -1;
				});
			});
		});
	},

	appendToPlaylist: function(tracks) {
		if (!Array.isArray(tracks)) {
			tracks = [tracks];
		}

		this.set('playlist', this.get('playlist').concat(tracks.map(function(track) {
			var mutTrack = Object.create(track);
			mutTrack.queuedby = 'user';
			return mutTrack;
		})));
	},

	removeFromPlaylist: function(trackIndex) {
		// Remove a track by id.
		if (typeof trackIndex !== 'number') {
			trackIndex = this.get('playlist').findIndex(function(elem) {
				return elem.id === trackIndex.id;
			});
		}
		if (trackIndex === -1) {
			return;
		}

		this.set('playlist', this.get('playlist').filter(function(t, i) {
			return i !== trackIndex;
		}));
	},
});
