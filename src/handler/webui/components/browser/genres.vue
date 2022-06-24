<template>
	<tab-view class="browser-genres" :tabs="tabs" @pop="tabPop">
		<template #genres>
			<h2>Genres</h2>
			<ul class="result-list">
				<li v-for="genre in genres"
					:class="{active: genre == selectedGenre}"
					@click="selectedArtist=''; selectedGenre=genre">{{ genre }}</li>
			</ul>
		</template>
		<template #artists>
			<h2>Artists</h2>
			<ul class="result-list">
				<li v-for="artist in artists"
					:class="{active: artist == shownArtist}"
					@click="selectedArtist = artist">{{ artist }}</li>
			</ul>
		</template>
		<template #tracks>
			<h2>Tracks</h2>
			<ul class="result-list">
				<li v-for="track in tracks" title="formatTrackTitle(track)"
					@click="appendToPlaylist(track, $event)">
					<span class="track-title">{{ track.title }}</span>
					<span class="track-duration">{{ durationToString(track.duration) }}</span>
					<span class="track-album">{{ track.album }}</span>
					<span class="glyphicon glyphicon-plus"></span>
				</li>
			</ul>
		</template>
	</tab-view>
</template>

<script>
	import ApiMixin from '../mixins/api.js';
	import PlaylistMixin from '../mixins/playlist.js';
	import TabView from './tab-view.vue';
	import TrackMixin from '../mixins/track.js';
	import { stringCompareCaseInsensitive } from '../mixins/util.js';

	export default {
		mixins: [ApiMixin, TrackMixin, PlaylistMixin],
		components: {
			TabView,
		},
		props: {
			library: {required: true, type: Array},
		},
		data() {
			return {selectedGenre: '', selectedArtist: ''};
		},
		computed: {
			shownArtist() {
				if (!this.selectedGenre) return '';
				if (this.artists.length == 1) return this.artists[0];
				return this.selectedArtist;
			},
			tree() {
				return this.library.reduce((genres, track) => {
					let genreTitle = track.genre || 'Unknown';
					let artistTitle = track.artist || 'Unknown';
					let artists = genres[genreTitle] || (genres[genreTitle] = {});
					let trackList =  artists[artistTitle] || (artists[artistTitle] = []);
					trackList.push(track);
					return genres;
				}, {});
			},
			genres() {
				return Object.keys(this.tree).sort(stringCompareCaseInsensitive);
			},
			artists() {
				if (!this.selectedGenre) return null;
				return Object.keys(this.tree[this.selectedGenre])
					.sort(stringCompareCaseInsensitive);
			},
			tracks() {
				if (!this.shownArtist) return null;
				return this.tree[this.selectedGenre][this.shownArtist].sort((a, b) => {
					return stringCompareCaseInsensitive(a.title, b.title);
				});
			},
			tabs() {
				let tabs = ['genres'];
				if (this.selectedGenre) {
					tabs.push('artists');
					if (this.shownArtist) {
						tabs.push('tracks');
					}
				}
				return tabs;
			},
		},
		methods: {
			tabPop(tab) {
				if (!this.selectedArtist || this.shownArtist != this.selectedArtist) {
					this.selectedGenre = '';
				}
				this.selectedArtist = '';
			},
		}
	}
</script>

<style lang="scss">
	.browser-genres {
		.tab > h2 {
			margin-top: 0;
			flex-shrink: 0;
		}

		@media (min-width: 992px) {
			.tab.tab-genres {
				width: 20%;
			}

			.tab.tab-artists,
			.tab.tab-tracks {
				width: 40%;
			}
		}
	}
</style>
