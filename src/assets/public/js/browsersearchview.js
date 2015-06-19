'use strict';

var BrowserSearchView = Backbone.View.extend({
	tagName:   'div',
	className: 'browser-view browser-search',

	events: {
		'input .search-input': 'doSearch',
		'click .do-show-more': 'doShowMore',
	},

	initialize: function(options) {
		this.listenTo(this.model, 'change:tracks', this.doSearch);
		this.render();
	},

	render: function() {
		this.$el.html(this.template());
		this.$('search-input').focus();
	},

	doSearch: function() {
		this.showResults(this.$('.search-input').val(), false);
	},

	doShowMore: function() {
		this.showResults(this.$('.search-input').val(), true);
	},

	showResults: function(query, showAll) {
		var $list = this.$('.result-list');
		$list.empty();

		if (!query) {
			return;
		}

		$list.addClass('all-shown');
		var results = this.model.search(query);
		if (results.length > 32 && !showAll) {
			results = results.slice(0, 32);
			$list.removeClass('all-shown');
		}

		var highlightExp = query.split(/\s+/).filter(function(kw) {
			return !!kw;
		}).map(function(kw) {
			// Escape the keyword into a HTML and then Regex safe string so it
			// won't cause any funny stuff.
			var safe = $('<span>').text(kw).html()
				.replace(/[\-\[\]\/\{\}\(\)\*\+\?\.\\\^\$\|]/g, '\\$&');
			return new RegExp('(>[^<>]*?)('+safe+')([^<>]*?<)', 'gi');
		});

		$list.append(results.map(function(track) {
			var self = this;

			var $el = $(highlightExp.reduce(function(html, re) {
				return html.replace(re, '$1<em>$2</em>$3');
			}, this.resultTemplate(track)));

			$el.on('click', function() {
				self.model.appendToPlaylist(track);
			});
			return $el;
		}, this));
	},

	template: _.template(
		'<div class="panel panel-default">'+
			'<div class="panel-body">'+
				'<div class="input-group">'+
					'<span class="input-group-addon">'+
						'<span class="glyphicon glyphicon-search"></span>'+
					'</span>'+
					'<input '+
						'autofocus '+
						'class="form-control input-lg search-input" '+
						'type="text" '+
						'placeholder="Search Everything" />'+
				'</div>'+
			'</div>'+
		'</div>'+
		'<div class="panel panel-default">'+
			'<div class="panel-body">'+
				'<ul class="result-list"></ul>'+
				'<button class="btn btn-default do-show-more">More</button>'+
			'</div>'+
		'</div>'
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
