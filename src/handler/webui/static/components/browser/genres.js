app.component('browser-genres', {
	mixins: [ApiMixin, TrackMixin, PlaylistMixin],
	props: {
		library: {required: true, type: Array},
	},
	data: function() {
		return {selectedGenre: '', selectedArtist: ''};
	},
	template: `
		<div class="browser-genres view tab-view">
			<div class="tab tab-name-genre">
				<h2>Genres</h2>
				<ul class="result-list">
					<li v-for="genre in genres"
						:class="{active: genre == selectedGenre}"
						@click="selectedArtist=''; selectedGenre=genre">{{ genre }}</li>
				</ul>
			</div>
			<div class="tab tab-name-artist" v-if="artists">
				<h2><a class="glyphicon glyphicon-arrow-left do-pop-tab" @click="pop()"></a>Artists</h2>
				<ul class="result-list">
					<li v-for="artist in artists"
						:class="{active: artist == shownArtist}"
						@click="selectedArtist = artist">{{ artist }}</li>
				</ul>
			</div>
			<div class="tab tab-name-artist" v-if="tracks">
				<h2><a class="glyphicon glyphicon-arrow-left do-pop-tab" @click="pop()"></a>Tracks</h2>
				<ul class="result-list">
					<li v-for="track in tracks" title="formatTrackTitle(track)"
						@click="appendToPlaylist(track, $event)">
						<span class="track-title">{{ track.title }}</span>
						<span class="track-duration">{{ durationToString(track.duration) }}</span>
						<span class="track-album">{{ track.album }}</span>
						<span class="glyphicon glyphicon-plus"></span>
					</li>
				</ul>
			</div>
		</div>
	`,
	computed: {
		shownArtist: function() {
			if (!this.selectedGenre) return '';
			if (this.artists.length == 1) return this.artists[0];
			return this.selectedArtist;
		},
		tree: function() {
			return this.library.reduce((genres, track) => {
				let genreTitle = track.genre || 'Unknown';
				let artistTitle = track.artist || 'Unknown';
				let artists = genres[genreTitle] || (genres[genreTitle] = {});
				let trackList =  artists[artistTitle] || (artists[artistTitle] = []);
				trackList.push(track);
				return genres;
			}, {});
		},
		genres: function() {
			return Object.keys(this.tree).sort(stringCompareCaseInsensitive);
		},
		artists: function() {
			if (!this.selectedGenre) return null;
			return Object.keys(this.tree[this.selectedGenre])
				.sort(stringCompareCaseInsensitive);
		},
		tracks: function() {
			if (!this.shownArtist) return null;
			return this.tree[this.selectedGenre][this.shownArtist].sort((a, b) => {
				return stringCompareCaseInsensitive(a.title, b.title);
			});
		},
	},
	methods: {
		pop: function() {
			if (!this.selectedArtist || this.shownArtist != this.selectedArtist) {
				this.selectedGenre = '';
			}
			this.selectedArtist = '';
		},
	}
});
