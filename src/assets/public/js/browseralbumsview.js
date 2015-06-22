'use strict';

var BrowserAlbumsView = Backbone.View.extend({
	tagName:   'div',
	className: 'browser-view browser-albums',

	initialize: function(options) {
		this.listenTo(this.model, 'change:tracks', this.render);
		this.render();
	},

	render: function() {
		this.$el.html(this.template());

		// Get a list of tracks which belong to an album.
		var albumTracks = this.model.get('tracks').filter(function(track) {
			return !!track.album && !!track.albumartist;
		});

		// Sort tracks into an artist/album tree structure.
		var artistAlbums = {};
		albumTracks.forEach(function(track) {
			var artist = artistAlbums[track.albumartist] || (artistAlbums[track.albumartist] = {});
			var album = artist[track.album] || (artist[track.album] = []);
			album.push(track);
		});

		// Flatten the tree into a list.
		this.albums = Object.keys(artistAlbums).sort().reduce(function(results, artistName) {
			return results.concat(Object.keys(artistAlbums[artistName]).sort().map(function(albumTitle) {
				return {
					title:  albumTitle,
					artist: artistName,
					tracks: artistAlbums[artistName][albumTitle],
				};
			}));
		}, []);

		this.$('.album-list ul')
			.empty()
			.lazyLoad(this.doLazyLoad, this);
		this.appendAlbums(24);
	},

	renderAlbum: function(album) {
		var self = this;

		album.sort(function(a, b) {
			return a.albumtrack > b.albumtrack ? 1
			: a.albumtrack < b.albumtrack ? -1
			: 0;
		});

		// Sort tracks into discs. If no disc data is available, all tracks are
		// stuffed into one disc.
		var discsObj = album.reduce(function(discs, track, i) {
			var disc = discs[track.albumdisc || ''] || (discs[track.albumdisc || ''] = []);
			var mutTrack = Object.create(track);
			mutTrack.selectionIndex = i; // Used for queueing the track when clicked.
			disc.push(mutTrack);
			return discs;
		}, {});

		// Make the disc data easier to process.
		var discs = Object.keys(discsObj).map(function(discTitle, i, discTitles) {
			return {
				// If only one disc is detected, why even bother showing the label?
				title:  discTitles.length > 1 ? discTitle : null,
				tracks: discsObj[discTitle],
			};
		});

		var $el = this.$('.album-view');
		$el.html(this.albumTemplate({
			title:    album[0].album,
			artist:   album[0].albumartist,
			duration: this.albumDuration(album),
			discs:    discs,
		}));

		showTrackArt($el.find('.album-art'), album[0]);
		$el.find('.album-info').on('click', function() {
			self.model.appendToPlaylist(album);
		});
		$el.find('.disc-title').on('click', function() {
			self.model.appendToPlaylist(discs[$(this).attr('data-index')].tracks);
		});
		$el.find('.result-list li.track').on('click', function() {
			self.model.appendToPlaylist(album[$(this).attr('data-index')]);
		});
	},

	appendAlbums: function(count) {
		var self = this;

		var $list = this.$('.album-list ul');
		var numChildren = $list.children().length;
		var albums = this.albums.slice(numChildren, numChildren + count);
		if (!albums.length) {
			return;
		}

		$list.append(albums.map(function(album) {
			var $el = $(this.albumPreviewTemplate({
				artist:   album.artist,
				title:    album.title,
				duration: this.albumDuration(album.tracks),
			}));
			showTrackArt($el.find('.track-art'), album.tracks[0], function(success) {
				$el.toggleClass('show-details', !success);
			});
			$el.on('click', function() {
				$list.find('li.active').removeClass('active');
				$el.addClass('active');
				self.renderAlbum(album.tracks);
			});
			return $el;
		}, this));
	},

	doLazyLoad: function() {
		this.appendAlbums(8);
	},

	albumDuration: function(tracks) {
		return tracks.reduce(function(total, track) {
			return total + track.duration;
		}, 0);
	},

	template: _.template(
		'<div class="album-list">'+
			'<h2>Albums</h2>'+
			'<ul class="result-list "></ul>'+
		'</div>'+
		'<div class="album-view"></div>'
	),
	albumPreviewTemplate:_.template(
		'<li title="<%- artist %> - <%- title %> (<%- durationToString(duration) %>)">'+
			'<div class="track-art">'+
				'<span class="album-artist"><%- artist %></span>'+
				'<span class="album-title"><%- title %></span>'+
			'</div>'+
		'</li>'
	),
	albumTemplate:_.template(
		'<div class="album-art"></div>'+
		'<div class="album-info">'+
			'<p>'+
				'<span class="album-title"><%- title %></span>'+
				'<span class="album-duration track-duration"><%- durationToString(duration) %></span>'+
				'<span class="album-artist"><%- artist %></span>'+
			'</p>'+
		'</div>'+
		'<ul class="result-list">'+
			'<% discs.forEach(function(disc, di) { %>'+
				'<% if (disc.title) { %>'+
					'<li class="disc-title" data-index="<%= di %>"><%- disc.title %></li>'+
				'<% } %>'+
				'<% disc.tracks.forEach(function(track) { %>'+
					'<li class="track" data-index="<%= track.selectionIndex %>">'+
						'<span class="track-num"><%- track.albumtrack %></span>'+
						'<span class="track-artist"><%- track.artist %></span>'+
						'<span class="track-title"><%- track.title %></span>'+
						'<span class="track-duration"><%- durationToString(track.duration) %></span>'+
					'</li>'+
				'<% }) %>'+
			'<% }) %>'+
		'</ul>'
	),
});
