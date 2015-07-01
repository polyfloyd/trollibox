'use strict';

var BrowserBrowseView = Backbone.View.extend({
	tagName:   'div',
	className: 'browser-view browser-browse',

	initialize: function(options) {
		this.listenTo(this.model, 'change:tracks', this.updateTree);
		this.updateTree();
	},

	updateTree: function() {
		this.genreTree = this.model.get('tracks').reduce(function(genres, track) {
			var genreTitle = track.genre || 'Unknown';
			var artists = genres[genreTitle] || (genres[genreTitle] = {});
			var trackList =  artists[track.artist] || (artists[track.artist] = []);
			trackList.push(track);
			return genres;
		}, {});

		this.render();
	},

	render: function() {
		this.$el.html(this.template());
		this.showGenreList();
	},

	showGenreList: function() {
		var self = this;

		var $tab = this.$('.genre-tab');
		$tab.html(this.genreTabTemplate({
			genres: Object.keys(this.genreTree).sort(stringCompareCaseInsensitive),
		}));
		$tab.find('.result-list li').on('click', function() {
			$tab.find('.result-list li.active').removeClass('active');
			var $li = $(this);
			$li.addClass('active');
			self.showArtistList($li.attr('data-genre'));
		});
	},

	showArtistList: function(genreTitle) {
		var self = this;

		var artists = Object.keys(this.genreTree[genreTitle])
			.sort(stringCompareCaseInsensitive);

		var $tab = this.$('.artist-tab');
		$tab.html(this.artistTabTemplate({ artists: artists }));
		$tab.find('.result-list li').on('click', function() {
			$tab.find('.result-list li.active').removeClass('active');
			var $li = $(this);
			$li.addClass('active');
			self.showTrackList(genreTitle, $li.attr('data-artist'));
		});

		if (artists.length === 1) {
			$tab.find('.result-list li').addClass('active');
			this.showTrackList(genreTitle, artists[0]);
		} else {
			this.$('.track-tab').empty();
		}
	},

	showTrackList: function(genreTitle, artistTitle) {
		var self = this;

		var $tab = this.$('.track-tab');
		$tab.html(this.trackTabTemplate({
			tracks: this.genreTree[genreTitle][artistTitle].sort(function(a, b) {
				return stringCompareCaseInsensitive(a.title, b.title);
			}),
		}));
		$tab.find('.result-list li').on('click', function() {
			var index = $(this).attr('data-index');
			self.model.appendToPlaylist(self.genreTree[genreTitle][artistTitle][index]);
		});
	},

	template: _.template(
		'<div class="genre-tab"></div>'+
		'<div class="artist-tab"></div>'+
		'<div class="track-tab"></div>'
	),
	genreTabTemplate: _.template(
		'<h2>Genres</h2>'+
		'<ul class="result-list">'+
			'<% genres.forEach(function(genre) { %>'+
				'<li data-genre="<%- genre %>"><%- genre %></li>'+
			'<% }) %>'+
		'</ul>'
	),
	artistTabTemplate: _.template(
		'<h2>Artists</h2>'+
		'<ul class="result-list">'+
			'<% artists.forEach(function(artist) { %>'+
				'<li data-artist="<%- artist %>"><%- artist %></li>'+
			'<% }) %>'+
		'</ul>'
	),
	trackTabTemplate: _.template(
		'<h2>Tracks</h2>'+
		'<ul class="result-list">'+
			'<% tracks.forEach(function(track, index) { %>'+
				'<li data-index="<%= index %>">'+
					'<span class="track-title"><%- track.title %></span>'+
					'<span class="track-duration"><%- durationToString(track.duration) %></span>'+
					'<span class="track-album"><%- track.album %></span>'+
				'</li>'+
			'<% }) %>'+
		'</ul>'
	),
});
