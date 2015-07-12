'use strict';

var BrowserStreamsView = Backbone.View.extend({
	tagName:   'div',
	className: 'browser-view browser-streams',

	initialize: function() {
		this.listenTo(this.model, 'change:streams', this.render);
		this.render();
	},

	render: function() {
		this.$el.html(this.template());

		var $list = this.$('.result-list');
		$list.append(this.model.get('streams').map(function(stream) {
			var self = this;
			var $el = $(this.streamTemplate({
				title: stream.album || stream.id,
			}));
			showTrackArt($el.find('.track-art'), stream);
			$el.on('click', function() {
				self.model.appendToPlaylist(stream);
			});
			return $el;
		}, this));
	},

	streamTemplate:_.template(
		'<li title="<%- title %>">'+
			'<div class="track-art">'+
				'<span class="stream-title"><%- title %></span>'+
			'</div>'+
		'</li>'
	),

	template: _.template(
		'<ul class="result-list"></ul>'
	),
});
