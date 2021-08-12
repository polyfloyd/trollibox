// To be used as argument to Array#sort(). Compares strings without case
// sensitivity.
function stringCompareCaseInsensitive(a, b) {
	a = a.toLowerCase();
	b = b.toLowerCase();
	return a > b ? 1
		: a < b ? -1
		: 0;
}

let ApiMixin = {
	props: {
		urlroot: {required: true, type: String},
		selectedPlayer: {required: true, type: String},
	},
};


var _formatTrackTitleTemplate = _.template(`<% if (albumtrack) {%><%= albumtrack %>.  <% } %><% if (artist) {%><%= artist %> - <% } %><%= title %><% if (duration) {%> (<%- duration %>)<% } %>`);

let TrackMixin = {
	methods: {
		formatTrackTitle: function(track) {
			return  _formatTrackTitleTemplate({
				artist: track.artist || '',
				title: track.title || '',
				albumtrack: track.albumtrack || '',
				duration: track.duration && this.durationToString(track.duration),
			});
		},
		durationToString: function(seconds) {
			let parts = [];
			for (let i = 1; i <= 3; i++) {
				let l = seconds % Math.pow(60, i - 1);
				parts.push((seconds % Math.pow(60, i) - l) / Math.pow(60, i - 1));
			}

			// Don't show hours if there are none.
			if (parts[2] === 0) {
				parts.pop();
			}

			return parts.reverse().map((p) => {
				return (p<10 ? '0' : '')+p;
			}).join(':');
		},
	}
};


let PlaylistMixin = {
	mixins: [ApiMixin],
	methods: {
		insertIntoPlaylist: async function(tracks, index, elems) {
			if (!Array.isArray(tracks)) tracks = [ tracks ];

			// Shows an animation to indicate that a track was added to the
			// playlist.
			$(elems || []).each((i, el) => {
				setTimeout(() => {
					var $elem = $(el);
					var $anim = $('<div class="insertion-animation glyphicon glyphicon-plus"></div>');
					$anim.css($elem.offset());

					$('body').prepend($anim);
					setTimeout(() => {
						$anim.remove();
					}, 1500);
				}, i * 40);
			});

			let res = await fetch(`${this.urlroot}data/player/${this.selectedPlayer}/playlist`, {
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
		},
		appendToPlaylist: function(tracks, elems) {
			return this.insertIntoPlaylist(tracks, -1, elems);
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
			if (this.playlist.length <= this.index + 1) return;
			let rem = [];
			for (let i = this.index+1; i < this.playlist.length; i++) {
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
};


let PlayerMixin = {
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
};