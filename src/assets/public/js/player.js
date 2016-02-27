'use strict';

var Player = Backbone.Model.extend({
	defaults: {
		'current':    -1,
		'playlist':   [],
		'time':       0,
		'queuerules': [],
		'state':      'stopped',
		'streams':    [],
		'tracks':     [],
		'volume':     0,
	},

	initialize: function(args) {
		this.reloaders = {};
		this.name      = args.name;

		this.attachServerReloader('server-event:playstate', '/player/'+this.name+'/playstate', function(data) {
			this.setInternal('state', data.playstate);
		});
		this.attachServerReloader('server-event:volume', '/player/'+this.name+'/volume', function(data) {
			this.setInternal('volume', data.volume);
		});
		this.attachServerReloader('server-event:time', '/player/'+this.name+'/time', function(data) {
			this.setInternal('time', data.time);
		});
		this.attachServerReloader('server-event:playlist', '/player/'+this.name+'/playlist', function(data) {
			var plist = data.tracks.map(this.fillMissingTrackFields, this);
			this.setInternal('playlist', plist);
			if (this.get('current') === data.current) {
				this.trigger('change:current');
			}
			this.setInternal('current', data.current);
			this.setInternal('time', data.time);
		});
		this.attachServerReloader('server-event:tracks', '/player/'+this.name+'/tracks', function(data) {
			this.setInternal('tracks', data.tracks.map(this.fillMissingTrackFields, this));
		});

		this.attachServerReloader('server-event:streams-update', '/streams', function(data) {
			this.setInternal('streams', data.streams.map(this.fillMissingTrackFields, this));
		});
		this.attachServerReloader('server-event:queuer-update', '/queuer', function(data) {
			this.setInternal('queuerules', data.queuerules);
		});

		this.attachServerUpdater('time', '/player/'+this.name+'/time', function(value) {
			return { time: value };
		});
		this.attachServerUpdater('state', '/player/'+this.name+'/playstate', function(value) {
			return { playstate: value };
		});
		this.attachServerUpdater('volume', '/player/'+this.name+'/volume', function(value) {
			return { volume: value };
		});
		this.attachServerUpdater('queuerules', '/queuer', function(value) {
			return { queuerules: value };
		});

		this.on('change:current', this.reloadProgressUpdater, this);
		this.on('change:state',   this.reloadProgressUpdater, this);
		this.on('server-connect', this.reload, this);
		this.connectEventSocket();
	},

	connectEventSocket: function() {
		var self = this;

		var proto = window.location.protocol.replace(/^http/, 'ws');
		var path = URLROOT.replace(/^https?:(\/\/)?/, '');
		if (path === '/') {
			path = window.location.host+path;
		}
		var sock = new WebSocket(proto+'//'+path+'data/player/'+this.name+'/listen');

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

	callServer: function(path, method, body, cb) {
		$.ajax({
			url:      URLROOT+'data'+path,
			method:   method,
			dataType: 'json',
			data:     body ? JSON.stringify(body) : null,
			context:  this,
		}).done(function(responseJson, status, res) {
			if (cb) cb.call(this, null, responseJson);
		}).fail(function(res, status, statusText) {
			var err = res.responseJSON && res.responseJSON.error
				? new Error(res.responseJSON.error)
				: new Error(res.responseText);
			this.trigger('error', err);
			if (cb) cb(err, null);
		});
	},

	attachServerReloader: function(event, path, handler) {
		var reload = function() {
			this.callServer(path, 'GET', null, function(err, data) {
				if (!err) handler.call(this, data);
			});
		};
		this.on(event, reload, this);
		this.reloaders[event] = reload;
	},

	attachServerUpdater: function(name, path, getUpdateData) {
		var waiting   = false;
		var nextValue = undefined;

		function update(value) {
			waiting = true;
			this.callServer(path, 'POST', getUpdateData.call(this, value), function(err, data) {
				if (err) {
					waiting = false;
					return;
				}
				setTimeout(function() {
					waiting = false;
					if (typeof nextValue !== 'undefined') {
						update.call(this, nextValue);
						nextValue = undefined;
					}
				}.bind(this), 200);
			});
		}

		this.on('change:'+name, function(obj, value, options) {
			if (options.sender === this) {
				return;
			}
			if (waiting) {
				nextValue = value;
			} else {
				update.call(this, value);
			}
		});
	},

	getCurrentTrack: function() {
		var c = this.get('current');
		if (c == -1) {
			return null;
		}
		return this.get('playlist')[c];
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

		clearInterval(this.timeUpdater);
		clearTimeout(this.timeTimeout);

		var cur = this.getCurrentTrack();
		if (cur && this.get('state') === 'playing') {
			this.timeUpdater = setInterval(function() {
				self.setInternal('time', self.get('time') + 1);
			}, 1000);
			if (cur.duration) {
				this.timeTimeout = setTimeout(function() {
					self.reload('server-event:player');
				}, 1000 * (cur.duration - this.get('time')));
			}
		}
	},

	next: function() {
		this.callServer('/player/'+this.name+'/next', 'POST');
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
		return track;
	},

	appendToPlaylist: function(tracks) {
		if (!Array.isArray(tracks)) {
			tracks = [tracks];
		}
		this.set('playlist', this.get('playlist').concat(tracks.map(function(tr) {
			var newTr = {};
			for (var k in tr) newTr[k] = tr[k];
			newTr.queuedby = 'user';
			return newTr;
		})));
		this.callServer('/player/'+this.name+'/playlist', 'PUT', {
			position: -1,
			tracks:   tracks.map(function(track) {
				return track.uri;
			}),
		});
	},

	removeFromPlaylist: function(trackIndices) {
		if (!Array.isArray(trackIndices)) {
			trackIndices = [trackIndices];
		}
		this.set('playlist', this.get('playlist').filter(function(tr, i) {
			return trackIndices.indexOf(i) === -1;
		}));
		this.callServer('/player/'+this.name+'/playlist', 'DELETE', {
			positions: trackIndices,
		});
	},

	moveInPlaylist: function(from, to) {
		var plist = this.get('playlist');
		plist.splice(to, 0, plist.splice(from, 1)[0]);
		this.set('playlist', plist);
		this.trigger('change:playlist');
		this.callServer('/player/'+this.name+'/playlist', 'PATCH', {
			from: from,
			to:   to,
		});
	},

	searchTracks: function(query, untagged, cb) {
		var path = '/player/'+this.name+'/tracks/search?query='+encodeURIComponent(query)+'&untagged='+encodeURIComponent(untagged.join(','));
		this.callServer(path, 'GET', null, function(err, data) {
			if (err) {
				cb(err, []);
				return;
			}
			data.tracks.forEach(function(res) {
				res.track = this.fillMissingTrackFields(res.track);
			}, this);
			cb(null, data.tracks);
		});
	},

	addStream: function(stream) {
		this.callServer('/streams', 'POST', { stream: stream });
	},

	removeStream: function(stream) {
		this.callServer('/streams', 'DELETE', { stream: stream })
	},

	addDefaultQueueRule: function() {
		this.set('queuerules', this.get('queuerules').concat([{
			attribute: 'artist',
			invert:    false,
			operation: 'contains',
			value:     '',
		}]));
	},

	playRawTracks: function(files) {
		files = Array.prototype.filter.call(files, function(file) {
			return file.type.match('^audio.+$');
		});
		if (!files.length) {
			return;
		}

		var form = new FormData();
		files.forEach(function(file) {
			form.append('files', file, file.name);
		});

		$.ajax({
			url:         URLROOT+'data/player/'+this.name+'/playlist/appendraw',
			method:      'POST',
			context:     this,
			data:        form,
			processData: false,
			contentType: false,
			error:       function(res, status, message) {
				var err = res.responseJSON && res.responseJSON.error
					? new Error(res.responseJSON.error)
					: new Error(message);
				this.trigger('error', err);
			},
		}).fail(function(res, status, statusText) {
			var err = res.responseJSON && res.responseJSON.error
				? new Error(res.responseJSON.error)
				: new Error(res.responseText);
			this.trigger('error', err);
		});
	},
});
