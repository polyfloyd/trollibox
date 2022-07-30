import ApiMixin from './api.js';
import PlaylistMixin from './playlist.js';

export default {
	mixins: [ApiMixin, PlaylistMixin],
	methods: {
		async setIndex(position, relative) {
			let res = await fetch(`${this.urlroot}data/player/${this.selectedPlayer}/current`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ current: position, relative: !!relative }),
			});
			if (res.status >= 400) {
				throw new Error('could not set current track index');
			}
		},
		async setTime(seconds) {
			let res = await fetch(`${this.urlroot}data/player/${this.selectedPlayer}/time`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ time: seconds }),
			});
			if (res.status >= 400) {
				throw new Error('could not set time');
			}
		},
		async setState(state) {
			let res = await fetch(`${this.urlroot}data/player/${this.selectedPlayer}/playstate`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ playstate: state }),
			});
			if (res.status >= 400) {
				throw new Error('could not set state');
			}
		},
		async setVolume(volume) {
			let res = await fetch(`${this.urlroot}data/player/${this.selectedPlayer}/volume`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ volume }),
			});
			if (res.status >= 400) {
				throw new Error('could not set volume');
			}
		},
		async setAutoQueuerFilter(filter) {
			let res = await fetch(`${this.urlroot}data/player/${this.selectedPlayer}/autoqueuer`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ filter }),
			});
			if (res.status >= 400) {
				throw new Error('could not set autoqueuer filter');
			}
		},
	},
}
