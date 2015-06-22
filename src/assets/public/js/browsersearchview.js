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
		this.$('search-input input').focus();
		this.$('.result-list').lazyLoad(this.doLazyLoad, this);
	},

	doSearch: function() {
		this.query   = this.$('.search-input input').val();
		this.results = this.model.search(this.query);
		this.$('.result-list').empty();
		this.appendResults(60);
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

		var highlightExp = this.query.split(/\s+/).filter(function(kw) {
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
		'<div class="search-input">'+
			'<div class="input-group">'+
				'<span class="input-group-addon">'+
					'<span class="glyphicon glyphicon-search"></span>'+
				'</span>'+
				'<input '+
					'autofocus '+
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
