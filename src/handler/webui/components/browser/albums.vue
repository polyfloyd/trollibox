<template>
	<div class="browser-albums view tab-view">
		<div class="tab tab-name-list">
			<h2>Albums</h2>
			<div class="grid-list">
				<div class="grid-item" v-for="(album, i) in albums" :key="i"
					:title="album.artist+' - '+album.title+' ('+durationToString(album.duration)+')'"
					@click="showAlbumByIndex(i)">
					<track-art :urlroot="urlroot" :selected-player="selectedPlayer" :track="album.tracks[0]" />
					<span class="album-artist">{{ album.artist }}</span>
					<span class="album-title">{{ album.title }}</span>
				</div>
			</div>
		</div>

		<div class="tab tab-name-album" v-if="detailAlbum">
			<track-art :urlroot="urlroot" :selected-player="selectedPlayer" :track="detailAlbum.tracks[0]" />
				<a class="glyphicon glyphicon-arrow-left do-pop-tab" @click="closeAlbumDetail()"></a>
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
		</div>
	</div>
</template>

<script>
	import PlaylistMixin from '../mixins/playlist.js';
	import TrackArt from '../track-art.vue';
	import TrackMixin from '../mixins/track.js';
	import { stringCompareCaseInsensitive } from '../mixins/util.js';

	export default {
		mixins: [TrackMixin, PlaylistMixin],
		components: {
			TrackArt,
		},
		props: {
			showAlbumByTrack: {type: Object},
			library: {required: true, type: Array},
		},
		data: function() {
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
		watch: {
			showAlbumByTrack(track) {
				this.reloadShowAlbumByTrack();
			},
		},
		methods: {
			closeAlbumDetail() {
				this.detailSelector = null;
			},
			showAlbumByIndex(i) {
				this.detailSelector = i;
			},
		},
	}
</script>

<style>
.browser-albums .tab {
	width: 50%;
	display: flex;
	flex-direction: column;
}

.browser-albums.tab-view .do-pop-tab {
	margin-bottom: 0.4em;
	font-size: 20px;
}

.browser-albums .tab-name-list h2 {
	flex-shrink: 0;
}

.browser-albums .tab-name-list .grid-item {
	cursor: pointer;
}

.browser-albums .tab-name-list .grid-item > .album-artist,
.browser-albums .tab-name-list .grid-item > .album-title {
	position: absolute;
	padding: 0 0.2em;
	opacity: 0;
	overflow: hidden;
	background-color: var(--color-bg);
}

.browser-albums .tab-name-list .grid-item > .track-art.placeholder ~ *,
.browser-albums .tab-name-list .grid-item:hover > * {
	opacity: 1;
}

.browser-albums .tab-name-list .grid-item > .album-title {
	top: 0;
	left: 0;
	right: 0;
}

.browser-albums .tab-name-list .grid-item > .album-artist {
	left: 0;
	right: 0;
	bottom: 0;
}

.browser-albums .tab.tab-name-album {
	position: relative;
	padding: 15px;
	overflow: hidden;
}

.browser-albums .tab-name-album {
	display: flex;
	flex-direction: column;
}

.browser-albums .tab-name-album .album-info {
	margin: 0;
	flex-shrink: 0;
	cursor: pointer;
}

.browser-albums .tab-name-album .album-content {
	overflow-y: auto;
}

.browser-albums .tab-name-album .album-disc-title {
	margin: 0.4em 0 0 0;
	font-size: 1.2em;
	cursor: pointer;
}

.browser-albums .tab-name-album .album-info,
.browser-albums .tab-name-album .album-disc-title,
.browser-albums .tab-name-album .result-list {
	padding: 0.5em;
}

.browser-albums .tab-name-album .album-info:hover,
.browser-albums .tab-name-album .album-info:hover + .album-content,
.browser-albums .tab-name-album .album-disc-title:hover,
.browser-albums .tab-name-album .album-disc-title:hover + .result-list,
.browser-albums .tab-name-album .result-list li:hover {
	background-color: var(--color-bg);
}

.browser-albums .tab-name-album .album-info:hover + .album-content .result-list > li > .glyphicon,
.browser-albums .tab-name-album .album-disc-title:hover + .result-list > li > .glyphicon {
	opacity: 1;
}

.browser-albums .tab-name-album .album-disc-title:before {
	content: "Disc";
	margin-right: 0.3em;
}

.browser-albums .tab-name-album .album-title {
	font-size: 1.4em;
}

.browser-albums .tab-name-album .album-duration {
	margin: 0.3em;
}

.browser-albums .tab-name-album .album-artist {
	display: block;
}

.browser-albums .tab-name-album .track-art {
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

.browser-albums .tab-name-album .track-art ~ * {
	z-index: 10;
}
</style>
