'use strict';

var Player = NetModel.extend({
	defaults: {
		'current':    -1,
		'playlist':   [],
		'time':       0,
		'state':      'stopped',
		'tracks':     [],
		'volume':     0,
	},

	initialize: function(args) {
		this.name = args.name;

		this.on('change:current', this.reloadProgressUpdater, this);
		this.on('change:state',   this.reloadProgressUpdater, this);
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

		this.attachServerUpdater('time', '/player/'+this.name+'/time', function(value) {
			return { time: value };
		});
		this.attachServerUpdater('state', '/player/'+this.name+'/playstate', function(value) {
			return { playstate: value };
		});
		this.attachServerUpdater('volume', '/player/'+this.name+'/volume', function(value) {
			return { volume: value };
		});
		NetModel.prototype.initialize.call(this, {
			eventSocketPath: '/player/'+this.name+'/listen',
		});
	},

	getCurrentTrack: function() {
		var c = this.get('current');
		if (c == -1) {
			return null;
		}
		return this.get('playlist')[c];
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
