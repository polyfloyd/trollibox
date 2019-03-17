'use strict';

var BrowserAlbumsView = BrowserView.extend({
	tagName:   'div',
	className: 'view browser-albums',

	initialize: function(options) {
		this.tabs = new TabView();
		this.$el.append(this.tabs.$el);
		this.listenTo(this.model, 'change:tracks', this.render);
		this.render();
	},

	render: function() {
		this.tabs.clearTabs();
		var $tab = this.tabs.pushTab($(this.albumListTemplate()), { name: 'list' });

		// Get a list of tracks which belong to an album.
		var albumTracks = this.model.get('tracks').filter((track) => {
			return !!track.album && !!track.albumartist;
		});

		// Sort tracks into an artist/album tree structure.
		var artistAlbums = {};
		albumTracks.forEach((track) => {
			var artist = artistAlbums[track.albumartist] || (artistAlbums[track.albumartist] = {});
			var album = artist[track.album] || (artist[track.album] = []);
			album.push(track);
		});

		// Flatten the tree into a list.
		var albums = Object.keys(artistAlbums)
			.sort(stringCompareCaseInsensitive)
			.reduce((albums, artistName) => {
				return Object.keys(artistAlbums[artistName])
					.sort(stringCompareCaseInsensitive)
					.reduce((albums, albumTitle) => {
						var album = artistAlbums[artistName][albumTitle];
						// Showing albums is pretty pointless and wastes screen
						// space with libraries that are not tagged very well.
						if (album.length <= 1) {
							return albums;
						}

						return albums.concat({
							title:  albumTitle,
							artist: artistName,
							tracks: album,
						});
					}, albums);
		}, []);

		var $list = $tab.find('ul');
		$list.append(albums.map((album) => {
			var $el = $(this.albumPreviewTemplate({
				artist:   album.artist,
				title:    album.title,
				duration: this.albumDuration(album.tracks),
			}));
			showTrackArt($el.find('.track-art'), this.model, album.tracks[0])
				.then((hasArt) => {
					$el.toggleClass('show-details', !hasArt);
				})
				.catch((err) => {
					$el.toggleClass('show-details', true);
					console.error(err);
				});
			$el.on('click', () => {
				$list.find('.active').removeClass('active');
				$el.addClass('active');
				this.renderAlbum(album.tracks);
			});
			return $el;
		}));
	},

	renderAlbum: function(album) {
		var self = this;

		album.sort((a, b) => {
			var at = a.albumtrack;
			var bt = b.albumtrack;
			// Add a zero padding to make sure '12' > '4'.
			while (at.length > bt.length) bt = '0'+bt;
			while (bt.length > at.length) at = '0'+at;
			return stringCompareCaseInsensitive(at, bt);
		});

		// Sort tracks into discs. If no disc data is available, all tracks are
		// stuffed into one disc.
		var discsObj = album.reduce((discs, track, i) => {
			var disc = discs[track.albumdisc || ''] || (discs[track.albumdisc || ''] = []);
			var mutTrack = Object.create(track);
			mutTrack.selectionIndex = i; // Used for queueing the track when clicked.
			disc.push(mutTrack);
			return discs;
		}, {});

		// Make the disc data easier to process.
		var discs = Object.keys(discsObj).map((discTitle, i, discTitles) => {
			return {
				// If only one disc is detected, why even bother showing the label?
				title:  discTitles.length > 1 ? discTitle : null,
				tracks: discsObj[discTitle],
			};
		});

		var $tab = this.tabs.pushTab($(this.albumTemplate({
			title:    album[0].album,
			artist:   album[0].albumartist,
			duration: this.albumDuration(album),
			discs:    discs,
		})), { name: 'album' });

		showTrackArt($tab.find('.album-art'), this.model, album[0]);
		$tab.find('.album-info').on('click', () => {
			showInsertionAnimation($tab.find('.result-list > li'));
			Hotkeys.playerInsert(self.model, album);
		});
		$tab.find('.album-disc-title').on('click', () => {
			var $discTitle = $(this);
			showInsertionAnimation($discTitle.next().find('> li'));
			Hotkeys.playerInsert(self.model, discs[$discTitle.attr('data-index')].tracks);
		});
		$tab.find('.result-list li.track').on('click', () => {
			var $el = $(this);
			showInsertionAnimation($el);
			Hotkeys.playerInsert(self.model, album[$el.attr('data-index')]);
		});
	},

	albumDuration: function(tracks) {
		return tracks.reduce((total, track) => total + track.duration, 0);
	},

	albumListTemplate: _.template(`
		<h2>Albums</h2>
		<ul class="result-list grid-list"></ul>
	`),
	albumPreviewTemplate:_.template(`
		<li title="<%- artist %> - <%- title %> (<%- durationToString(duration) %>)">
			<img class="ratio" src="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAABmJLR0QA/wD/AP+gvaeTAAAACXBIWXMAAAsTAAALEwEAmpwYAAAADUlEQVQI12NgYGBgAAAABQABXvMqOgAAAABJRU5ErkJggg==" />
			<div class="track-art">
				<span class="album-artist"><%- artist %></span>
				<span class="album-title"><%- title %></span>
			</div>
		</li>
	`),
	albumTemplate: _.template(`
		<div class="album-art"></div>
		<a class="glyphicon glyphicon-arrow-left do-pop-tab"></a>
		<p class="album-info">
			<span class="album-title"><%- title %></span>
			<span class="album-duration track-duration"><%- durationToString(duration) %></span>
			<span class="album-artist"><%- artist %></span>
		</p>
		<div class="album-content">
			<% discs.forEach((disc, di) => { %>
				<% if (disc.title) { %>
					<p class="album-disc-title" data-index="<%= di %>"><%- disc.title %></p>
				<% } %>
				<ul class="result-list">
					<% disc.tracks.forEach((track) => { %>
						<li class="track" data-index="<%= track.selectionIndex %>" title="<%- formatTrackTitle(track) %>">
							<span class="track-num"><%- track.albumtrack %></span>
							<span class="track-artist"><%- track.artist %></span>
							<span class="track-title"><%- track.title %></span>
							<span class="track-duration"><%- durationToString(track.duration) %></span>
							<span class="glyphicon glyphicon-plus"></span>
						</li>
					<% }) %>
				</ul>
			<% }) %>
		</div>
	`),
});
