'use strict';

var Player = Backbone.Model.extend({
	defaults: {
		'current':    null,
		'playlist':   [],
		'progress':   0,
		'queuerules': [],
		'state':      'stopped',
		'streams':    [],
		'tracks':     [],
		'volume':     0,
	},

	initialize: function(args) {
		this.reloaders = {};
		this.name      = args.name;

		this.attachServerReloader('server-event:playstate', 'data/player/'+this.name+'/playstate', function(data) {
			this.setInternal('state', data.playstate);
		});
		this.attachServerReloader('server-event:volume', 'data/player/'+this.name+'/volume', function(data) {
			this.setInternal('volume', data.volume);
		});
		this.attachServerReloader('server-event:playlist', 'data/player/'+this.name+'/playlist', function(data) {
			var pl = data.tracks.map(this.fillMissingTrackFields, this);
			this.setInternal('playlist', pl);
			if (pl.length > 0 && (!this.get('current') || pl[0].id != this.get('current').id)) {
				this.setInternal('current', pl[0]);
				this.setInternal('progress', data.tracks[0].progress);
			}
			if (pl.length == 0) {
				console.log('set current null')
				this.setInternal('current', null);
				this.setInternal('progress', 0);
			}
		});
		this.attachServerReloader('server-event:tracks', 'data/player/'+this.name+'/tracks', function(data) {
			this.setInternal('tracks', data.tracks.map(this.fillMissingTrackFields, this));
		});

		this.attachServerReloader('server-event:streams-update', 'data/streams', function(data) {
			this.setInternal('streams', data.streams.map(this.fillMissingTrackFields, this));
		});
		this.attachServerReloader('server-event:queuer-update', 'data/queuer', function(data) {
			this.setInternal('queuerules', data.queuerules);
		});

		this.attachServerUpdater('progress', 'data/player/'+this.name+'/progress', function(value) {
			return { progress: value };
		});
		this.attachServerUpdater('state', 'data/player/'+this.name+'/playstate', function(value) {
			return { playstate: value };
		});
		this.attachServerUpdater('volume', 'data/player/'+this.name+'/volume', function(value) {
			return { volume: value };
		});
		this.attachServerUpdater('playlist', 'data/player/'+this.name+'/playlist', function(value) {
			return {
				'track-ids': value.map(function(track) {
					return track.id;
				}),
			};
		});
		this.attachServerUpdater('queuerules', 'data/queuer', function(value) {
			return { queuerules: value };
		});

		this.on('change:current', this.reloadProgressUpdater, this);
		this.on('change:state',   this.reloadProgressUpdater, this);
		this.on('server-connect', this.reload, this);
		this.connectEventSocket();
	},

	connectEventSocket: function() {
		var self = this;

		var wsRoot = URLROOT.replace(/^http/, 'ws');
		var sock = new WebSocket(wsRoot+'data/player/'+this.name+'/listen');
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

	attachServerReloader: function(event, path, handle) {
		var reload = function() {
			$.ajax({
				url:      URLROOT+path,
				method:   'GET',
				dataType: 'json',
				context:  this,
				success:  function(data) {
					handle.call(this, data);
				},
				error:    function(req, str, err) {
					this.trigger('error', err);
				},
			});
		};
		this.on(event, reload, this);
		this.reloaders[event] = reload;
	},

	attachServerUpdater: function(name, path, getUpdateData) {
		var self = this;

		var waiting   = false;
		var nextValue = undefined;

		function update(value, cb) {
			waiting = true;
			$.ajax({
				url:      URLROOT+path,
				method:   'POST',
				dataType: 'json',
				data:     JSON.stringify(getUpdateData.call(self, value)),
				success:  function() {
					setTimeout(function() {
						waiting = false;
						if (typeof nextValue !== 'undefined') {
							update(nextValue);
							nextValue = undefined;
						}
					}, 200);
				},
				error:    function(res, status, message) {
					waiting = false;
					var err = cb(res.responseJSON.error || new Error(message));
					self.trigger('error', err);
					self.trigger('error:'+name, err);
				},
			});
		}

		this.on('change:'+name, function(obj, value, options) {
			if (options.sender === this) {
				return;
			}
			if (waiting) {
				nextValue = value;
			} else {
				update(value);
			}
		});
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

	reload: function(name) {
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

	reloadProgressUpdater: function() {
		var self = this;

		clearInterval(this.progressUpdater);
		clearTimeout(this.progressTimeout);

		if (this.get('current') && this.get('state') === 'playing') {
			this.progressUpdater = setInterval(function() {
				self.setInternal('progress', self.get('progress') + 1);
			}, 1000);
			if (this.get('current').duration) {
				this.progressTimeout = setTimeout(function() {
					self.reload('server-event:player');
				}, 1000 * (this.get('current').duration - this.get('progress')));
			}
		}
	},

	next: function() {
		$.ajax({
			url:      URLROOT+'data/player/'+this.name+'/next',
			method:   'POST',
			dataType: 'json',
			context:  this,
			error:    function(req, str, err) {
				this.trigger('error', err);
			},
		});
	},

	fillMissingTrackFields: function(track) {
		// Ensure every field is a string.
		[
			'artist',
			'title',
			'genre',
			'album',
			'albumartist',
			'albumtrack',
			'albumdisc',
		].forEach(function(k) {
			track[k] || (track[k] = '');
		});

		if (!track.title || !track.artist) {
			// First, attempt to find an "<artist> - <title>" string.
			var artistAndTitle = [
				'title',
				'artist',
			].reduce(function(ant, attr) {
				if (ant || !track[attr]) {
					return ant;
				}
				return track[attr].match(/(.+)\s+-\s+(.+)/);
			}, null);
			// Also try the filename. We use a different regex to cut off the
			// path and extension.
			artistAndTitle = artistAndTitle || track.id.match(/^(?:.*\/)?(.+)\s+-\s+(.+)\.\w+$/);
			if (artistAndTitle) {
				track.artist = artistAndTitle[1];
				track.title  = artistAndTitle[2];
			} else {
				// If that doesn't work, use the filename or stream URL.
				track.title = track.id.match(/^https?:\/\//)
					? track.id // Use the stream URL.
					: function(t) { //  Cut the filename from the path.
						return t ? t[1] : '';
					}(track.id.match(/^(?:.*\/)?(.+)\.\w+$/));
			}
		}
		return track;
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

	addStream: function(stream) {
		$.ajax({
			url:      URLROOT+'data/streams',
			method:   'POST',
			dataType: 'json',
			data:     JSON.stringify({ stream: stream }),
			context:  this,
			success:  function() {
				this.reload('server-event:streams-update');
			},
			error:    function(req, str, err) {
				this.trigger('error', err);
			},
		});
	},

	removeStream: function(stream) {
		$.ajax({
			url:      URLROOT+'data/streams',
			method:   'DELETE',
			dataType: 'json',
			context:  this,
			data:     JSON.stringify({ stream: stream }),
			success:  function() {
				this.reload('server-event:streams-update');
			},
			error:    function(req, str, err) {
				this.trigger('error', err);
			},
		});
	},

	loadDefaultStreams: function() {
		$.ajax({
			url:      URLROOT+'data/streams/loaddefault',
			method:   'POST',
			dataType: 'json',
			context:  this,
			success:  function() {
				this.reload('server-event:streams-update');
			},
			error:    function(req, str, err) {
				this.trigger('error', err);
			},
		});
	},

	addDefaultQueueRule: function() {
		this.set('queuerules', this.get('queuerules').concat([{
			attribute: 'artist',
			invert:    false,
			operation: 'contains',
			value:     '',
		}]));
	},
});
