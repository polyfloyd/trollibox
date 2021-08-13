Vue.component('browser-albums', {
	mixins: [TrackMixin, PlaylistMixin],
	props: {
		library: {required: true, type: Array},
	},
	data: function() {
		return {detailAlbumIndex: -1};
	},
	template: `
		<div class="browser-albums view tab-view">
			<div class="tab tab-name-list">
				<h2>Albums</h2>
				<div class="grid-list">
					<div class="grid-item" v-for="(album, i) in albums" :key="i"
						:title="album.artist+' - '+album.title+' ('+durationToString(album.duration)+')'"
						@click="detailAlbumIndex = i">
						<track-art :urlroot="urlroot" :selected-player="selectedPlayer" :track="album.tracks[0]" />
						<span class="album-artist">{{ album.artist }}</span>
						<span class="album-title">{{ album.title }}</span>
					</div>
				</div>
			</div>

			<div class="tab tab-name-album" v-if="detailAlbum">
				<track-art :urlroot="urlroot" :selected-player="selectedPlayer" :track="detailAlbum.tracks[0]" />
				<a class="glyphicon glyphicon-arrow-left do-pop-tab" @click="detailAlbumIndex = -1"></a>
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
	`,
	computed: {
		albums: function() {
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
		detailAlbum: function() {
			if (this.detailAlbumIndex == -1) return null;

			let album = this.albums[this.detailAlbumIndex];

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
});
