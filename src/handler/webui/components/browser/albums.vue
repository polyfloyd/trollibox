<template>
	<tab-view class="browser-albums" :tabs="!detailAlbum ? ['list'] : ['list', 'detail']" @pop="tabPop">
		<template #list>
			<h2>Albums</h2>
			<div class="grid-list album-list">
				<div class="grid-item" v-for="(album, i) in albums" :key="i"
					:title="album.artist+' - '+album.title+' ('+durationToString(album.duration)+')'"
					@click="showAlbumByIndex(i)">
					<track-art :urlroot="urlroot" :selected-player="selectedPlayer" :track="album.tracks[0]" />
					<span class="album-artist">{{ album.artist }}</span>
					<span class="album-title">{{ album.title }}</span>
				</div>
			</div>
		</template>
		<template #detail>
			<track-art class="album-art-background" :urlroot="urlroot" :selected-player="selectedPlayer" :track="detailAlbum.tracks[0]" />
			<p class="album-info" @click="appendToPlaylist(detailAlbum.tracks, $event)">
				<span class="album-title">{{ detailAlbum.title }}</span>
				<span class="album-duration track-duration">{{ durationToString(detailAlbum.duration) }}</span>
				<span class="album-artist">{{ detailAlbum.artist }}</span>
			</p>
			<div class="album-content">
				<template v-for="disc in detailAlbum.discs">
					<p v-if="disc.title" class="album-disc-title" @click="appendToPlaylist(disc.tracks, $event)">{{ disc.title }}</p>
					<ul class="result-list">
						<li v-for="track in disc.tracks" class="track" :title="formatTrackTitle(track)" @click="appendToPlaylist(track, $event)">
							<span class="track-num">{{ track.albumtrack }}</span>
							<span class="track-artist">{{ track.artist }}</span>
							<span class="track-title">{{ track.title }}</span>
							<span class="track-duration">{{ durationToString(track.duration) }}</span>
							<span class="glyphicon glyphicon-plus"></span>
						</li>
					</ul>
				</template>
			</div>
		</template>
	</tab-view>
</template>

<script>
	import PlaylistMixin from '../mixins/playlist.js';
	import TabView from './tab-view.vue';
	import TrackArt from '../track-art.vue';
	import TrackMixin from '../mixins/track.js';
	import { stringCompareCaseInsensitive } from '../mixins/util.js';

	export default {
		mixins: [TrackMixin, PlaylistMixin],
		components: {
			TabView,
			TrackArt,
		},
		props: {
			showAlbumByTrack: {type: Object},
			library: {required: true, type: Array},
		},
		data() {
			return {detailSelector: this.showAlbumByTrack};
		},
		computed: {
			detailIndex() {
				if (this.detailSelector === null) {
					return -1;

				} else if (typeof this.detailSelector == 'number') {
					return this.detailSelector;

				} else if (typeof this.detailSelector == 'object') {
					let {album, albumartist} = this.detailSelector;
					return this.albums.findIndex(item => {
						return item.artist == albumartist && item.title == album;
					});
				}
				throw new Error('invalid selector type');
			},
			albums() {
				// Get a list of tracks which belong to an album.
				let albumTracks = this.library.filter(track => track.album && track.albumartist);

				// Sort tracks into an artist/album tree structure.
				let artistAlbums = {};
				albumTracks.forEach(track => {
					let artist = artistAlbums[track.albumartist] || (artistAlbums[track.albumartist] = {});
					let album = artist[track.album] || (artist[track.album] = []);
					album.push(track);
				});

				// Flatten the tree into a list.
				let albums = Object.keys(artistAlbums)
					.sort(stringCompareCaseInsensitive)
					.reduce((albums, artistName) => {
						return Object.keys(artistAlbums[artistName])
							.sort(stringCompareCaseInsensitive)
							.reduce((albums, albumTitle) => {
								let album = artistAlbums[artistName][albumTitle];
								// Showing albums is pretty pointless and wastes screen
								// space with libraries that are not tagged very well.
								if (album.length <= 1) return albums;

								return albums.concat({
									title: albumTitle,
									artist: artistName,
									tracks: album,
									duration: album.reduce((t, track) => t + track.duration, 0),
								});
							}, albums);
					}, []);
				return albums;
			},
			detailAlbum() {
				if (this.detailIndex == -1) return null;

				let album = this.albums[this.detailIndex];

				album.tracks.sort((a, b) => {
					let at = a.albumtrack || '';
					let bt = b.albumtrack || '';
					// Add a zero padding to make sure '12' > '4'.
					while (at.length > bt.length) bt = '0'+bt;
					while (bt.length > at.length) at = '0'+at;
					return stringCompareCaseInsensitive(at, bt);
				});

				// Sort tracks into discs. If no disc data is available, all tracks are
				// stuffed into one disc.
				let discsObj = album.tracks.reduce((discs, track, i) => {
					let disc = discs[track.albumdisc || ''] || (discs[track.albumdisc || ''] = []);
					let mutTrack = Object.create(track);
					mutTrack.selectionIndex = i; // Used for queueing the track when clicked.
					disc.push(mutTrack);
					return discs;
				}, {});

				// Make the disc data easier to process.
				let discs = Object.keys(discsObj).map((discTitle, i, discTitles) => {
					return {
						// If only one disc is detected, do not show the label.
						title:  discTitles.length > 1 ? discTitle : null,
						tracks: discsObj[discTitle],
					};
				});

				return {...album, discs};
			},
		},
		methods: {
			tabPop() {
				this.detailSelector = null;
			},
			showAlbumByIndex(i) {
				this.detailSelector = i;
			},
		},
	}
</script>

<style lang="scss">
	.browser-albums .tab-list {
		.tab > h2 {
			margin-top: 0;
		}

		.grid-item {
			cursor: pointer;
		}

		.grid-item > .album-artist,
		.grid-item > .album-title {
			position: absolute;
			padding: 0 0.2em;
			opacity: 0;
			overflow: hidden;
			background-color: var(--color-bg);
		}

		.grid-item > .track-art.placeholder ~ *,
		.grid-item:hover > * {
			opacity: 1;
		}

		.grid-item > .album-title {
			top: 0;
			left: 0;
			right: 0;
		}

		.grid-item > .album-artist {
			left: 0;
			right: 0;
			bottom: 0;
		}
	}

	.browser-albums .tab-detail {
		position: relative;
		padding: 15px;
		overflow: hidden;
		display: flex;
		flex-direction: column;

		.album-info {
			margin: 0;
			flex-shrink: 0;
			cursor: pointer;
		}

		.album-content {
			overflow-y: auto;
		}

		.album-disc-title {
			margin: 0.4em 0 0 0;
			font-size: 1.2em;
			cursor: pointer;
		}

		.album-info,
		.album-disc-title,
		.result-list {
			padding: 0.5em;
		}

		.album-info:hover,
		.album-info:hover + .album-content,
		.album-disc-title:hover,
		.album-disc-title:hover + .result-list,
		.result-list li:hover {
			background-color: var(--color-bg);
		}

		.album-info:hover + .album-content .result-list > li > .glyphicon,
		.album-disc-title:hover + .result-list > li > .glyphicon {
			opacity: 1;
		}

		.album-disc-title:before {
			content: "Disc";
			margin-right: 0.3em;
		}

		.album-title {
			font-size: 1.4em;
		}

		.album-duration {
			margin: 0.3em;
		}

		.album-artist {
			display: block;
		}

		.album-art-background {
			position: absolute;
			top: -4px;
			left: -4px;;
			right: -4px;
			bottom: -4px;
			z-index: 0;
			opacity: 0.4;
			background-position: 50%;
			background-size: cover;
			background-repeat: no-repeat;
			-webkit-filter: blur(4px);
			-moz-filter: blur(4px);
			filter: blur(4px);
		}

		.album-art-background ~ * {
			z-index: 10;
		}
	}
</style>
