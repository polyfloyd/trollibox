'use strict';

var BrowserSearchView = Backbone.View.extend({
	tagName:   'div',
	className: 'browser-view browser-search',

	events: {
		'input .search-input input': 'doSearch',
	},

	initialize: function(options) {
		this.listenTo(this.model, 'change:tracks', this.doSearch);
		this.render();
	},

	render: function() {
		this.$el.html(this.template());
		this.$('.result-list').lazyLoad(this.doLazyLoad, this);
	},

	focusInput: function() {
		this.trigger('request-focus');
		this.$('.search-input input').focus();
	},

	doSearch: function() {
		var self = this;

		var query = this.query();
		if (query.length <= 1) {
			this.$('.result-list').empty();
			return;
		}

		if (this.searchInProgress) {
			return;
		}
		this.searchInProgress = true;

		searchTracks(query, this.model.get('tracks'), function(list) {
			self.searchInProgress = false;
			if (query != self.query()) {
				self.doSearch();
				return;
			}

			self.$('.result-list').empty();
			self.results = list.sort(function(a, b) {
				var matchesCmp = a.matches > b.matches ? -1
					: a.matches < b.matches ? 1
					: 0;
				if (matchesCmp !== 0) {
					return matchesCmp;
				}

				var artistCmp = stringCompareCaseInsensitive(a.track.artist, b.track.artist);
				if (artistCmp !== 0) {
					return artistCmp;
				}

				var titleCmp = stringCompareCaseInsensitive(a.track.title, b.track.title);
				if (titleCmp !== 0) {
					return titleCmp;
				}

				var albumCmp = stringCompareCaseInsensitive(a.track.album, b.track.album);
				if (albumCmp !== 0) {
					return albumCmp;
				}
			});
			self.appendResults(60);
		});
	},

	query: function() {
		return this.$('.search-input input').val();
	},

	doLazyLoad: function() {
		this.appendResults(20);
	},

	appendResults: function(count) {
		var $list = this.$('.result-list');

		var numChildren = $list.children().length;
		var results = this.results.slice(numChildren, numChildren + count);
		if (!results.length) {
			return;
		}

		var highlightExp = this.query().split(/\s+/).filter(function(kw) {
			return !!kw;
		}).map(function(kw) {
			// Escape the keyword into a HTML and then Regex safe string so it
			// won't cause any funny stuff.
			var safe = $('<span>').text(kw).html()
				.replace(/[\-\[\]\/\{\}\(\)\*\+\?\.\\\^\$\|]/g, '\\$&');
			return new RegExp('(>[^<>]*?)('+safe+')([^<>]*?<)', 'gi');
		});

		$list.append(results.map(function(result) {
			var self = this;

			var $el = $(highlightExp.reduce(function(html, re) {
				return html.replace(re, '$1<em>$2</em>$3');
			}, this.resultTemplate(result.track)));

			$el.on('click', function() {
				self.model.appendToPlaylist(result.track);
			});
			return $el;
		}, this));
	},

	template: _.template(
		'<div class="search-input">'+
			'<div class="input-group">'+
				'<span class="input-group-addon">'+
					'<span class="glyphicon glyphicon-search"></span>'+
				'</span>'+
				'<input '+
					'class="form-control input-lg" '+
					'type="text" '+
					'placeholder="Search Everything" />'+
			'</div>'+
		'</div>'+
		'<ul class="result-list search-results"></ul>'
	),
	resultTemplate: _.template(
		'<li>'+
			'<span class="track-artist"><%- artist %></span>'+
			'<span class="track-title"><%- title %></span>'+
			'<span class="track-duration"><%- durationToString(duration) %></span>'+
			'<span class="track-album"><%- album %></span>'+
		'</li>'
	),
});
