class StreamLibraryChangedEvent extends Event {
	constructor() {
		super('change:streams');
	}
}

class StreamDB extends EventTarget {
	constructor(args) {
		super();
		this.streams = [];

		this._ev = new EventSource(`${URLROOT}data/streams/events`);
		this._ev.onopen = () => {
			// Reload all state to ensure that we are in sync.
			this._reload();
		};
		this._ev.addEventListener('library:tracks', async event => {
			this.streams = await this._loadStreams();
			this.dispatchEvent(new StreamLibraryChangedEvent());
		});
		this._reload();
	}

	async remove(stream) {
		const filename = encodeURIComponent(stream.filename);
		const res = await fetch(`${URLROOT}data/streams?filename=${filename}`, {
			method: 'DELETE',
		});
		if (!res.ok) {
			throw new Error('Unable to remove stream');
		}
	}

	async add(stream) {
		const res = await fetch(`${URLROOT}data/streams`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ stream }),
		});
		if (!res.ok) {
			throw new Error('Unable to add stream');
		}
	}

	async _loadStreams() {
		const res = await fetch(`${URLROOT}data/streams`);
		if (!res.ok) {
			throw new Error('Unable to list streams');
		}
		const { streams } = await res.json();
		return streams.map(stream => {
			stream.uri = stream.url;
			return Player.fillMissingTrackFields(stream);
		});
	}

	async _reload() {
		this.streams = await this._loadStreams();
		this.dispatchEvent(new StreamLibraryChangedEvent());
	}
}
