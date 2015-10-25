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

	initialize: function() {
		this.reloaders = {};

		this.attachServerReloader('server-event:mixer', 'data/player/volume', function(data) {
			this.setInternal('volume', data.volume);
		});
		this.attachServerReloader('server-event:player', 'data/player/current', function(data) {
			this.setInternal('current',  data.track ? this.fillMissingTrackFields(data.track) : null);
			this.setInternal('progress', data.progress);
			this.setInternal('state',    data.state);
		});
		this.attachServerReloader('server-event:playlist', 'data/player/playlist', function(data) {
			this.setInternal('playlist', data.tracks.filter(function(track) {
				return !!track.id;
			}).map(this.fillMissingTrackFields, this));
		});
		this.attachServerReloader('server-event:update', 'data/track/browse/', function(data) {
			this.setInternal('tracks', data.tracks.map(this.fillMissingTrackFields, this));
		});
		this.attachServerReloader('server-event:streams-update', 'data/streams', function(data) {
			this.setInternal('streams', data.streams.map(this.fillMissingTrackFields, this));
		});
		this.attachServerReloader('server-event:queuer-update', 'data/queuer', function(data) {
			this.setInternal('queuerules', data.queuerules);
		});

		this.attachServerUpdater('progress', 'data/player/progress', function(value) {
			return { progress: value };
		});
		this.attachServerUpdater('state', 'data/player/state', function(value) {
			return { state: value };
		});
		this.attachServerUpdater('volume', 'data/player/volume', function(value) {
			return { volume: value };
		});
		this.attachServerUpdater('playlist', 'data/player/playlist', function(value) {
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
		var waiting   = false;
		var nextValue = undefined;
		this.on('change:'+name, function(obj, value, options) {
			if (options.sender === this) {
				return;
			}
			if (!waiting) {
				$.ajax({
					url:      URLROOT+path,
					method:   'POST',
					dataType: 'json',
					data:     JSON.stringify(getUpdateData.call(this, value)),
					context:  this,
					success:  function() {
						var self = this;
						setTimeout(function() {
							waiting = false;
							if (typeof nextValue !== 'undefined') {
								self.set(name, nextValue);
								nextValue = undefined;
							}
						}, 200);
					},
					error:    function(res, status, message) {
						waiting = false;
						var err = res.responseJSON.error || new Error(message);
						this.trigger('error', err);
						this.trigger('error:'+name, err);
					},
				});
				waiting = true;

			} else {
				nextValue = value;
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
			this.reloaders[name].call(this);
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

		return this.get('tracks').reduce(function(list, track) {
			var numMatches = 0;
			var allMatch = keywords.every(function(keyword) {
				return ['artist', 'title', 'album'].filter(function(attr) {
					var val = track[attr];
					if (typeof val === 'undefined') {
						return false;
					}
					var match = val.toLowerCase().indexOf(keyword) !== -1;
					numMatches += match ? 1 : 0; //  Meh, no functional for you!
					return match;
				}).length > 0;
			});

			if (allMatch) {
				// A concat() would be more correcter from a functional
				// perspective, but also A LOT slower! :(
				list.push({
					matches: numMatches,
					track:   track,
				});
				return list;
			} else {
				return list
			}
		}, []);
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

	addDefaultQueueRule: function() {
		this.set('queuerules', this.get('queuerules').concat([{
			attribute: 'artist',
			invert:    false,
			operation: 'contains',
			value:     '',
		}]));
	},
});
