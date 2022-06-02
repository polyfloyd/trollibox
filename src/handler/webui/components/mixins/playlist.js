import ApiMixin from '../mixins/api.js';

export default {
	mixins: [ApiMixin],
	methods: {
		insertIntoPlaylist: async function(tracks, index, event) {
			if (!Array.isArray(tracks)) tracks = [ tracks ];

			// Shows an animation to indicate that a track was added to the
			// playlist.
			let anim = document.createElement('div');
			anim.classList.add('insertion-animation');
			anim.classList.add('glyphicon');
			anim.classList.add('glyphicon-plus');
			anim.style.top = `calc(${event.y}px - 0.5em)`;
			anim.style.left = `calc(${event.x}px - 0.5em)`;

			document.body.appendChild(anim);
			setTimeout(() => {
				document.body.removeChild(anim)
			}, 1500);

			let at = event.ctrlKey ? 'Next'
				: index == -1 ? 'End'
				: null;

			let res = await fetch(`${this.urlroot}data/player/${this.selectedPlayer}/playlist`, {
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					at,
					position: index,
					tracks: tracks.map(track => track.uri),
				}),
			});
			if (res.status >= 400) {
				throw new Error('could not insert into playlist');
			}
		},
		appendToPlaylist: function(tracks, event) {
			return this.insertIntoPlaylist(tracks, -1, event);
		},
		removeFromPlaylist: async function(trackIndices) {
			if (!Array.isArray(trackIndices)) trackIndices = [ trackIndices ];

			let res = await fetch(`${this.urlroot}data/player/${this.selectedPlayer}/playlist`,{
				method: 'DELETE',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ positions: trackIndices }),
			});
			if (res.status >= 400) {
				throw new Error('could not remove from playlist');
			}
		},
		clearPlaylist: function() {
			if (this.playlist.length <= this.currentIndex + 1) return;
			let rem = [];
			for (let i = this.currentIndex+1; i < this.playlist.length; i++) {
				rem.push(i);
			}
			this.removeFromPlaylist(rem);
		},
		moveInPlaylist: async function(from, to) {
			let res = await fetch(`${this.urlroot}data/player/${this.selectedPlayer}/playlist`, {
				method: 'PATCH',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ from, to }),
			});
			if (res.status >= 400) {
				throw new Error('could not move in playlist');
			}
		},
	},
}
