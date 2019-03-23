class CurrentTrackChangedEvent extends Event {
	constructor(index) {
		super('change:index');
		this.index = index;
	}
}

class StateChangedEvent extends Event {
	constructor(state) {
		super('change:state');
		this.state = state;
	}
}

class TimeChangedEvent extends Event {
	constructor(time) {
		super('change:time');
		this.time = time;
	}
}

class VolumeChangedEvent extends Event {
	constructor(volume) {
		super('change:volume');
		this.volume = volume;
	}
}

class PlaylistChangedEvent extends Event {
	constructor(playlist) {
		super('change:playlist');
		this.playlist = playlist;
	}
}

class LibraryChangedEvent extends Event {
	constructor() {
		super('change:tracks');
	}
}

class Player extends EventTarget {
	constructor(name) {
		super();
		this.name = name;

		this.playlist = [];
		this.index = -1;
		this.state = 'stopped';
		this.time = 0;
		this.volume = 0;
		this.tracks = [];

		this._ev = new EventSource(`${URLROOT}data/player/${this.name}/events`);
		this._ev.onopen = () => {
			// Reload all state to ensure that we are in sync.
			this._reload();
		};
		this._ev.addEventListener('playlist', async event => {
			const { index, playlist } = await this._loadPlaylist();
			this.index = index;
			this.playlist = playlist;
			this.dispatchEvent(new CurrentTrackChangedEvent(this.index));
			this.dispatchEvent(new PlaylistChangedEvent(this.playlist));
		});
		this._ev.addEventListener('playstate', event => {
			this.state = JSON.parse(event.data).state;
			this._reloadProgressUpdater();
			this.dispatchEvent(new StateChangedEvent(this.state));
		});
		this._ev.addEventListener('time', event => {
			this.time = JSON.parse(event.data).time;
			this._reloadProgressUpdater();
			this.dispatchEvent(new TimeChangedEvent(this.time));
		});
		this._ev.addEventListener('volume', event => {
			this.volume = JSON.parse(event.data).volume;
			this.dispatchEvent(new VolumeChangedEvent(this.volume));
		});
		this._ev.addEventListener('tracks', async event => {
			this.tracks = await this._loadTrackLibrary();
			this.dispatchEvent(new LibraryChangedEvent());
		});
		this._reload();
	}

	getCurrentTrack() {
		if (this.index == -1) {
			return null;
		}
		return this.playlist[this.index];
	}

	async setIndex(position, relative) {
		const res = await fetch(`${URLROOT}data/player/${this.name}/current`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({
				current: position,
				relative: !!relative,
			}),
		});
		if (res.status >= 400) {
			throw new Error('could not set current track index');
		}
	}

	async setTime(seconds) {
		const res = await fetch(`${URLROOT}data/player/${this.name}/time`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ time: seconds }),
		});
		if (res.status >= 400) {
			throw new Error('could not set time');
		}
	}

	async setState(state) {
		const res = await fetch(`${URLROOT}data/player/${this.name}/playstate`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ playstate: state }),
		});
		if (res.status >= 400) {
			throw new Error('could not set state');
		}
	}

	async setVolume(volume) {
		const res = await fetch(`${URLROOT}data/player/${this.name}/volume`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ volume }),
		});
		if (res.status >= 400) {
			throw new Error('could not set volume');
		}
	}

	async insertIntoPlaylist(tracks, index) {
		if (!Array.isArray(tracks)) {
			tracks = [ tracks ];
		}

		const res = await fetch(`${URLROOT}data/player/${this.name}/playlist`, {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({
				position: index,
				tracks: tracks.map(track => track.uri),
			}),
		});
		if (res.status >= 400) {
			throw new Error('could not insert into playlist');
		}
	}

	appendToPlaylist(tracks) {
		return this.insertIntoPlaylist(tracks, -1);
	}

	async removeFromPlaylist(trackIndices) {
		if (!Array.isArray(trackIndices)) {
			trackIndices = [ trackIndices ];
		}

		const res = await fetch(`${URLROOT}data/player/${this.name}/playlist`,{
			method: 'DELETE', 
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ positions: trackIndices }),
		});
		if (res.status >= 400) {
			throw new Error('could not insert into playlist');
		}
	}

	async moveInPlaylist(from, to) {
		const res = await fetch(`${URLROOT}data/player/${this.name}/playlist`, {
			method: 'PATCH',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ from, to }),
		});
		if (res.status >= 400) {
			throw new Error('could not insert into playlist');
		}
	}

	async searchTracks(query, untagged) {
		const encUt = encodeURIComponent(untagged.join(','));
		const encQ = encodeURIComponent(query);
		const res = await fetch(`${URLROOT}data/player/${this.name}/tracks/search?query=${encQ}&untagged=${encUt}`);
		if (res.status >= 400) {
			throw new Error('could not insert into playlist');
		}

		const { tracks } = await res.json();
		for (let res of tracks) {
			res.track = Player.fillMissingTrackFields(res.track);
		}
		return tracks;
	}

	async playRawTracks(files) {
		files = Array.prototype.filter.call(files, file => {
			return file.type.match(/^audio/);
		});
		if (!files.length) {
			throw new Error('No files specified');
		}

		var form = new FormData();
		for (let file of files) {
			form.append('files', file, file.name);
		}
		const res = await fetch(`${URLROOT}data/player/${this.name}/playlist/appendraw`, {
			method: 'POST',
			body: form,
		});
		if (res.status >= 400) {
			throw new Error('could not play from network');
		}
	}

	async playFromNetwork(url) {
		if (!url.match(/^https?:\/\/.+/)) {
			throw new Error('Invalid URL');
		}
		const res = await fetch(`${URLROOT}data/player/${this.name}/playlist/appendnet`, {
			metod: 'POST',
			url,
		});
		if (res.status >= 400) {
			throw new Error('could not play from network');
		}
	}

	async _reload() {
		const [
			{ index, playlist },
			state,
			time,
			volume,
		] = await Promise.all([
			this._loadPlaylist(),
			this._loadState(),
			this._loadTime(),
			this._loadVolume(),
		]);
		this.index = index;
		this.playlist = playlist;
		this.state = state;
		this.time = time;
		this.volume = volume;

		this.dispatchEvent(new CurrentTrackChangedEvent(this.index));
		this.dispatchEvent(new PlaylistChangedEvent(this.playlist));
		this.dispatchEvent(new StateChangedEvent(this.state));
		this.dispatchEvent(new TimeChangedEvent(this.time));
		this.dispatchEvent(new VolumeChangedEvent(this.volume));

		this.tracks = await this._loadTrackLibrary();
		this.dispatchEvent(new LibraryChangedEvent());
	}

	async _loadPlaylist() {
		const res = await fetch(`${URLROOT}data/player/${this.name}/playlist`);
		if (res.status >= 400) {
			throw new Error('could not fetch tracks');
		}
		const { current, tracks } = await res.json();
		return {
			index: current,
			playlist: tracks.map(Player.fillMissingTrackFields),
		};
	}

	async _loadState() {
		const res = await fetch(`${URLROOT}data/player/${this.name}/playstate`);
		if (res.status >= 400) {
			throw new Error('could not fetch tracks');
		}
		const { playstate } = await res.json();
		return playstate;
	}

	async _loadTime() {
		const res = await fetch(`${URLROOT}data/player/${this.name}/time`);
		if (res.status >= 400) {
			throw new Error('could not fetch tracks');
		}
		const { time } = await res.json();
		return time;
	}

	async _loadVolume() {
		const res = await fetch(`${URLROOT}data/player/${this.name}/volume`);
		if (res.status >= 400) {
			throw new Error('could not fetch tracks');
		}
		const { volume } = await res.json();
		return volume;
	}

	async _loadTrackLibrary() {
		const res = await fetch(`${URLROOT}data/player/${this.name}/tracks`);
		if (res.status >= 400) {
			throw new Error('could not fetch tracks');
		}
		const { tracks } = await res.json();
		return tracks.map(Player.fillMissingTrackFields);
	}

	_reloadProgressUpdater() {
		clearInterval(this._timeUpdater);
		clearTimeout(this._timeTimeout);
		if (this.index != -1 && this.state === 'playing') {
			this.dispatchEvent(new TimeChangedEvent(this.time));
			this._timeUpdater = setInterval(() => {
				this.time += 1;
				this.dispatchEvent(new TimeChangedEvent(this.time));
			}, 1000);
		}
	}

	// fillMissingTrackFields ensures that all a track's optional properties
	// are set.
	static fillMissingTrackFields(track) {
		const props = [
			'artist',
			'title',
			'genre',
			'album',
			'albumartist',
			'albumtrack',
			'albumdisc',
		];
		for (const k of props) {
			track[k] || (track[k] = '');
		}
		return track;
	}
}
