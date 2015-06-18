'use strict';

var BrowserAlbumsView = Backbone.View.extend({
	tagName:   'div',
	className: 'browser-view browser-albums',

	initialize: function(options) {
		this.listenTo(this.model, 'change:tracks', this.render);
		this.render();
	},

	render: function() {
		var self = this;

		this.$el.html(this.template());
		var albumTracks = (this.model.get('tracks') || []).filter(function(track) {
			return !!track.album && !!track.albumartist;
		});

		var artistAlbums = {};
		albumTracks.forEach(function(track) {
			var artist = artistAlbums[track.albumartist] || (artistAlbums[track.albumartist] = {});
			var album = artist[track.album] || (artist[track.album] = []);
			album.push(track);
		});

		var $artistList = this.$('.artist-list ul');
		$artistList.empty();
		$artistList.append(Object.keys(artistAlbums).reduce(function(results, artistName) {
			return results.concat(Object.keys(artistAlbums[artistName]).map(function(albumName) {
				var album = artistAlbums[artistName][albumName];
				var $el = $(self.artistTemplate({
					artist:   artistName,
					title:    albumName,
					duration: self.albumDurationString(album),
				}));
				$el.on('click', function() {
					self.renderAlbum(album);
				});
				return $el;
			}));
		}, []));
	},

	renderAlbum: function(album) {
		var self = this;

		album.sort(function(a, b) {
			return a.albumtrack > b.albumtrack ? 1
			: a.albumtrack < b.albumtrack ? -1
			: 0;
		});

		var $el = this.$('.album-view');
		$el.html(this.albumTemplate({
			tracks:   album.map(function(track) {
				var mutTrack = Object.create(track);
				mutTrack.duration = durationToString(track.duration);
				return mutTrack;
			}),
			title:    album[0].album,
			artist:   album[0].albumartist,
			duration: this.albumDurationString(album),
		}));

		var art = 'url(\''+URLROOT+'data/track/art/'+encodeURIComponent(album[0].id).replace('\'', '\\\'')+'\')';
		$el.find('.track-art').css('background-image', art);

		$el.find('.result-list li').on('click', function() {
			self.model.appendToPlaylist(album[$(this).index()]);
		});
		$el.find('.album-info').on('click', function() {
			self.model.appendToPlaylist(album);
		});
	},

	albumDurationString: function(tracks) {
		return durationToString(tracks.reduce(function(total, track) {
			return total + track.duration;
		}, 0));
	},

	template: _.template(
		'<div class="panel panel-default">'+
			'<div class="panel-body">'+
				'<div class="row">'+
					'<div class="col-md-6 artist-list">'+
						'<h2>Albums</h2>'+
						'<ul class="result-list "></ul>'+
					'</div>'+
					'<div class="col-md-6 album-view"></div>'+
				'</div>'+
			'</div>'+
		'</div>'
	),
	artistTemplate:_.template(
		'<li>'+
			'<span class="track-artist"><%- artist %></span>'+
			'<span class="track-title"><%- title %></span>'+
			'<span class="track-duration"><%- duration %></span>'+
		'</li>'
	),
	albumTemplate:_.template(
		'<div class="track-art"></div>'+
		'<div class="album-info">'+
			'<p>'+
				'<span class="album-title"><%- title %></span>'+
				'<span class="album-duration track-duration"><%- duration %></span>'+
				'<span class="album-artist"><%- artist %></span>'+
			'</p>'+
		'</div>'+
		'<ul class="result-list">'+
		'<% tracks.forEach(function(track) { %>'+
			'<li>'+
				'<span class="track-num"><%- track.albumtrack %></span>'+
				'<span class="track-artist"><%- track.artist %></span>'+
				'<span class="track-title"><%- track.title %></span>'+
				'<span class="track-duration"><%- track.duration %></span>'+
			'</li>'+
		'<% }) %>'+
		'</ul>'
	),
});
