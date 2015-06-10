'use strict';

var PlayerView = Backbone.View.extend({
	tagName:   'div',
	className: 'player',

	initialize: function() {
		this.$el.html(this.template());

		this.listenTo(this.model, 'change:current',  this.renderCurrent);
		this.listenTo(this.model, 'change:playlist', this.renderPlaylist);
		this.listenTo(this.model, 'change:progress', this.renderProgress);
		this.listenTo(this.model, 'change:state',    this.renderState);

		this.renderCurrent();
		this.renderPlaylist();
		this.renderProgress();
		this.renderState();
	},

	renderCurrent: function() {
		var cur = this.model.get('current') || {};
		if (cur.art) {
			this.$('.track-art')
				.css('background-image', 'url(\''+cur.art+'\')')
				.removeClass('album-art-default');
		} else {
			this.$('.track-art')
				.css('background-image', '')
				.addClass('album-art-default');
		}
		this.$('.track-album').text(cur.album || '');
		this.$('.track-artist').text(cur.artist || '');
		this.$('.track-title').text(cur.title || '');
		this.$('.track-duration .total').text(cur.duration ? this.durationToString(cur.duration) : '');
	},

	renderProgress: function() {
		var pr = this.model.get('progress');
		this.$('.track-duration .current').text(pr ? this.durationToString(pr) : '');
	},

	renderState: function() {
		var state = this.model.get('state');
		this.$el.toggleClass('player-paused',  state === 'paused');
		this.$el.toggleClass('player-playing', state === 'playing');
		this.$el.toggleClass('player-stopped', state === 'stopped');
	},

	renderPlaylist: function() {
		var playlist = this.model.get('playlist') || [];
		if (playlist.length > 0) {
			// Slice off the currently playing track
			playlist = playlist.slice(1);
		}

		this.$('.player-playlist')
			.empty()
			.append(playlist.map(function(track) {
				return this.playlistTemplate(track);
			}, this));
	},

	durationToString: function(seconds) {
		var s = '';
		var hasHours = seconds > 3600;
		if (hasHours) {
			s += Math.round(seconds / 3600)+':';
			seconds %= 3600;
		}
		var min = Math.round(seconds / 60 - 0.5);
		if (min < 10 && hasHours) {
			s += '0';
		}
		s += min+':';
		var sec = seconds % 60;
		if (sec < 10) {
			s += '0';
		}
		return s + sec;
	},

	template: _.template(
		'<div class="player-now-playing">'+
			'<div class="track-art"></div>'+
			'<p class="track-album"></p>'+
			'<p>'+
				'<span class="track-artist"></span> - <span class="track-title"></span>'+
			'</p>'+
			'<p class="track-duration">'+
				'<span class="current"></span> / <span class="total"></span>'+
			'</p>'+
		'</div>'+

		'<div class="player-controls">'+
			'<button class="player-do-pause glyphicon glyphicon-pause"></button>'+
			'<button class="player-do-play glyphicon glyphicon-play"></button>'+
			'<button class="glyphicon glyphicon-forward"></button>'+
		'</div>'+

		'<ol class="player-playlist"></ol>'
	),
	playlistTemplate: _.template(
		'<li>'+
			'<button class="glyphicon glyphicon-remove"></button>'+
			'<span class="track-artist"><%- artist %></span> - <span class="track-name"><%- title %></span>'+
		'</li>'
	),

});
