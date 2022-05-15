export default {
	methods: {
		formatTrackTitle: function(track) {
			let s = '';
			if (track.albumtrack) s += `${track.albumtrack}. `;
			if (track.artist) s += `${track.artist} - `;
			s += track.title;
			if (track.duration) s += ` (${this.durationToString(track.duration)})`;
			return s;
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
}
