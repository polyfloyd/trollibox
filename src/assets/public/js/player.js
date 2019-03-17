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

		NetModel.prototype.initialize.call(this, {
			eventSourcePath: '/player/'+this.name+'/events',
		});

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
	},

	getCurrentTrack: function() {
		var c = this.get('current');
		if (c == -1) {
			return null;
		}
		return this.get('playlist')[c];
	},

	reloadProgressUpdater: function() {
		clearInterval(this.timeUpdater);
		clearTimeout(this.timeTimeout);

		var cur = this.getCurrentTrack();
		if (cur && this.get('state') === 'playing') {
			this.setInternal('time', this.get('time'));
			this.timeUpdater = setInterval(() => {
				this.setInternal('time', this.get('time') + 1);
			}, 1000);
			if (cur.duration) {
				this.timeTimeout = setTimeout(() => {
					this.reload('server-event:player');
				}, 1000 * (cur.duration - this.get('time')));
			}
		}
	},

	setCurrent: function(position, relative) {
		this.callServer('/player/'+this.name+'/current', 'POST', {
			current:  position,
			relative: !!relative,
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
		return track;
	},

	insertIntoPlaylist: function(tracks, index) {
		if (!Array.isArray(tracks)) {
			tracks = [tracks];
		}

		var insertTracks = tracks.map(function(tr) {
			var newTr = {};
			for (var k in tr) newTr[k] = tr[k];
			newTr.queuedby = 'user';
			return newTr;
		});
		var plist = this.get('playlist');
		var newPlist = index != -1
			? plist.slice(0, index).concat(insertTracks).concat(plist.slice(index))
			: plist.concat(insertTracks);
		this.set('playlist', newPlist);

		this.callServer('/player/'+this.name+'/playlist', 'PUT', {
			position: index,
			tracks:   tracks.map(function(track) { return track.uri; }),
		}).catch(() => {
			this.set('playlist', plist);
		});
	},

	appendToPlaylist: function(tracks) {
		this.insertIntoPlaylist(tracks, -1);
	},

	removeFromPlaylist: function(trackIndices) {
		if (!Array.isArray(trackIndices)) {
			trackIndices = [trackIndices];
		}
		var plist = this.get('playlist');
		this.set('playlist', plist.filter(function(tr, i) {
			return trackIndices.indexOf(i) === -1;
		}));
		this.callServer('/player/'+this.name+'/playlist', 'DELETE', {
			positions: trackIndices,
		}).catch(() => {
			this.set('playlist', plist);
		});
	},

	moveInPlaylist: function(from, to) {
		var plist = this.get('playlist');
		var oldPlist = this.get('playlist').map((e) => e);
		plist.splice(to, 0, plist.splice(from, 1)[0]);
		this.set('playlist', plist);
		this.trigger('change:playlist');
		this.callServer('/player/'+this.name+'/playlist', 'PATCH', {
			from: from,
			to:   to,
		}).catch(() => {
			this.set('playlist', oldPlist);
		});
	},

	searchTracks: function(query, untagged) {
		var encUt = encodeURIComponent(untagged.join(','));
		var encQ = encodeURIComponent(query);
		var path = `/player/${this.name}/tracks/search?query=${encQ}&untagged=${encUt}`;
		return this.callServer(path, 'GET', null)
			.then((data) => {
				data.tracks.forEach((res) => {
					res.track = this.fillMissingTrackFields(res.track);
				}, this);
				return Promise.resolve(data.tracks);
			});
	},

	playRawTracks: function(files) {
		files = Array.prototype.filter.call(files, function(file) {
			return file.type.match('^audio.+$');
		});
		if (!files.length) {
			return Promise.reject(new Error('No files specified'));
		}

		var form = new FormData();
		files.forEach(function(file) {
			form.append('files', file, file.name);
		});

		return fetch(`${URLROOT}data/player/${this.name}/playlist/appendraw`, {
			method: 'POST',
			body:   form,
		});
	},

	playFromNetwork: function(url) {
		if (!url.match(/^https?:\/\/.+/)) {
			return new Promise(function(resolve, reject) {
				reject(new Error('Invalid URL'));
			});
		}
		return this.callServer('/player/'+this.name+'/playlist/appendnet', 'POST', {
			url: url,
		});
	},
});
