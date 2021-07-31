'use strict';

var BrowserBrowseView = BrowserView.extend({
	tagName:   'div',
	className: 'view browser-browse',

	initialize: function(options) {
		this.tabs = new TabView();
		this.$el.append(this.tabs.$el);
		this.model.addEventListener('change:tracks', this.updateTree.bind(this));
		this.updateTree();
	},

	updateTree: function() {
		this.genreTree = this.model.tracks.reduce(function(genres, track) {
			var genreTitle = track.genre || 'Unknown';
			var artists = genres[genreTitle] || (genres[genreTitle] = {});
			var trackList =  artists[track.artist] || (artists[track.artist] = []);
			trackList.push(track);
			return genres;
		}, {});

		this.render();
	},

	render: function() {
		this.tabs.clearTabs();
		this.showGenreList();
	},

	showGenreList: function() {
		var self = this;

		var $tab = this.tabs.pushTab($(this.genreTabTemplate({
			genres: Object.keys(this.genreTree).sort(stringCompareCaseInsensitive),
		})), { name: 'genre' });
		$tab.find('.result-list > li').on('click', function() {
			$tab.find('.result-list > li.active').removeClass('active');
			var $li = $(this);
			$li.addClass('active');
			self.showArtistList($li.attr('data-genre'));
		});
	},

	showArtistList: function(genreTitle) {
		var self = this;

		var artists = Object.keys(this.genreTree[genreTitle])
			.sort(stringCompareCaseInsensitive);

		var $tab = this.tabs.pushTab($(this.artistTabTemplate({
			artists: artists,
		})), { name: 'artist' });

		$tab.find('.result-list li').on('click', function() {
			$tab.find('.result-list li.active').removeClass('active');
			var $li = $(this);
			$li.addClass('active');
			self.showTrackList(genreTitle, $li.attr('data-artist'));
		});

		if (artists.length === 1) {
			$tab.find('.result-list li').addClass('active');
			this.showTrackList(genreTitle, artists[0]);
		}
	},

	showTrackList: function(genreTitle, artistTitle) {
		var self = this;

		var $tab = this.tabs.pushTab($(this.trackTabTemplate({
			tracks: this.genreTree[genreTitle][artistTitle].sort(function(a, b) {
				return stringCompareCaseInsensitive(a.title, b.title);
			}),
		})), { name: 'track' });
		$tab.find('.result-list li').on('click', function() {
			let $li = $(this);
			var index = $li.attr('data-index');
			showInsertionAnimation($li);
			Hotkeys.playerInsert(self.model, self.genreTree[genreTitle][artistTitle][index]);
		});
	},

	genreTabTemplate: _.template(`
		<h2>Genres</h2>
		<ul class="result-list">
			<% genres.forEach(function(genre) { %>
				<li data-genre="<%- genre %>"><%- genre %></li>
			<% }) %>
		</ul>'
	`),
	artistTabTemplate: _.template(`
		<h2><a class="glyphicon glyphicon-arrow-left do-pop-tab"></a>Artists</h2>
		<ul class="result-list">
			<% artists.forEach(function(artist) { %>
				<li data-artist="<%- artist %>"><%- artist %></li>
			<% }) %>
		</ul>'
	`),
	trackTabTemplate: _.template(`
		<h2><a class="glyphicon glyphicon-arrow-left do-pop-tab"></a>Tracks</h2>
		<ul class="result-list">
			<% tracks.forEach(function(track, index) { %>
				<li data-index="<%= index %>" title="<%- formatTrackTitle(track) %>">
					<span class="track-title"><%- track.title %></span>
					<span class="track-duration"><%- durationToString(track.duration) %></span>
					<span class="track-album"><%- track.album %></span>
					<span class="glyphicon glyphicon-plus"></span>
				</li>
			<% }) %>
		</ul>'
	`),
});
