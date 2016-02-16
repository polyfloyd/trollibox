'use strict';

var BrowserSearchView = Backbone.View.extend({
	tagName:   'div',
	className: 'view browser-search',

	events: {
		'input .search-input input': 'doSearch',
	},

	initialize: function(options) {
		this.listenTo(this.model, 'change:tracks', this.doSearch);
		this.on('search-begin', function() {
			this.$el.addClass('search-busy');
		});
		this.on('search-complete', function() {
			this.$el.removeClass('search-busy');
		});
		this.render();
	},

	render: function() {
		this.$el.html(this.template());
	},

	focusInput: function() {
		this.trigger('request-focus');
		this.$('.search-input input').focus();
	},

	doSearch: function() {
		var self = this;

		var query = this.query();
		this.$('.result-list').empty();
		if (query.length <= 1) {
			return;
		}

		if (this.searchInProgress) {
			return;
		}
		this.trigger('search-begin');
		this.searchInProgress = true;
		this.model.searchTracks(query, ['artist', 'title', 'album'], function(err, results) {
			self.searchInProgress = false;
			if (query != self.query()) {
				self.doSearch();
				return;
			}

			self.trigger('search-complete');
			self.$('.result-list').lazyLoad(results, self.renderResult, self);
		});
	},

	query: function() {
		return this.$('.search-input input').val();
	},

	renderResult: function(result) {
		function highlight(result, property) {
			var m = result.matches[property];
			if (!m) {
				return _.escape(result.track[property]);
			}
			var value = m.sort(function(a, b) {
				return a.start > b.start;
			}).reduceRight(function(value, match) {
				return value.substring(0, match.start)+'<em>'+value.substring(match.start, match.end)+'</em>'+value.substring(match.end);
			}, result.track[property]);
			return _.escape(value).replace(/&lt;(\/)?em&gt;/g, '<$1em>');
		}

		var $el = $(this.resultTemplate({
			result:    result,
			highlight: highlight,
		}));
		$el.on('click', function() {
			this.model.appendToPlaylist(result.track);
		}.bind(this));
		return $el;
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
			'<span class="track-artist"><%= highlight(result, \'artist\') %></span>'+
			'<span class="track-title"><%= highlight(result, \'title\') %></span>'+
			'<span class="track-duration"><%- durationToString(result.track.duration) %></span>'+
			'<span class="track-album"><%= highlight(result, \'album\') %></span>'+
		'</li>'
	),
});
