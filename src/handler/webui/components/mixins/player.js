import ApiMixin from './api.js';
import PlaylistMixin from './playlist.js';

export default {
	mixins: [ApiMixin, PlaylistMixin],
	methods: {
		setIndex: async function(position, relative) {
			let res = await fetch(`${this.urlroot}data/player/${this.selectedPlayer}/current`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ current: position, relative: !!relative }),
			});
			if (res.status >= 400) {
				throw new Error('could not set current track index');
			}
		},
		setTime: async function(seconds) {
			let res = await fetch(`${this.urlroot}data/player/${this.selectedPlayer}/time`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ time: seconds }),
			});
			if (res.status >= 400) {
				throw new Error('could not set time');
			}
		},
		setState: async function(state) {
			let res = await fetch(`${this.urlroot}data/player/${this.selectedPlayer}/playstate`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ playstate: state }),
			});
			if (res.status >= 400) {
				throw new Error('could not set state');
			}
		},
		setVolume: async function(volume) {
			let res = await fetch(`${this.urlroot}data/player/${this.selectedPlayer}/volume`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ volume }),
			});
			if (res.status >= 400) {
				throw new Error('could not set volume');
			}
		},
	},
}
