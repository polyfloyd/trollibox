'use strict';

var PlayerView = Backbone.View.extend({
	tagName:   'div',
	className: 'player',

	events: {
		'click .do-next':          'doNext',
		'click .do-toggle-state':  'doToggleState',
		'click .do-toggle-volume': 'doToggleVolume',
		'input .do-set-volume':    'doSetVolume',
		'input .do-set-progress':  'doSetProgress',
	},

	initialize: function() {
		this.listenTo(this.model, 'change:current',  this.renderCurrent);
		this.listenTo(this.model, 'change:playlist', this.renderPlaylist);
		this.listenTo(this.model, 'change:progress', this.renderProgress);
		this.listenTo(this.model, 'change:state',    this.renderState);
		this.listenTo(this.model, 'change:volume',   this.renderVolume);
		this.render();
	},

	render: function() {
		this.$el.html(this.template());
		this.renderCurrent();
		this.renderPlaylist();
		this.renderProgress();
		this.renderState();
		this.renderVolume();
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
		this.$('.player-now-playing')
			.removeClass('queuedby-system queuedby-user')
			.addClass('queuedby-'+cur.queuedby);
		this.$('.track-duration .total').text(cur.duration ? durationToString(cur.duration) : '');
		this.$('.do-set-progress').attr('max', cur.duration);
	},

	renderProgress: function() {
		var pr = this.model.get('progress');
		var text = this.model.get('current') ? durationToString(pr) : '';
		this.$('.track-duration .current').text(text);
		this.$('.do-set-progress').val(pr || 0);
	},

	renderState: function() {
		var state = this.model.get('state');
		this.$el.toggleClass('player-paused',  state === 'paused');
		this.$el.toggleClass('player-playing', state === 'playing');
		this.$el.toggleClass('player-stopped', state === 'stopped');

		this.$('.do-toggle-state')
			.toggleClass('glyphicon-pause', state === 'playing')
			.toggleClass('glyphicon-play',  state !== 'playing');
	},

	renderVolume: function() {
		var vol = this.model.get('volume') || 0;
		this.$('.do-toggle-volume')
			.toggleClass('glyphicon-volume-off', vol === 0)
			.toggleClass('glyphicon-volume-up', vol > 0);

		var $setVol = this.$('.do-set-volume');
		$setVol.val(vol * parseInt($setVol.attr('max'), 10));
	},

	renderPlaylist: function() {
		var playlist = this.model.get('playlist') || [];
		if (playlist.length > 0) {
			// Slice off the currently playing track
			playlist = playlist.slice(1);
		}

		this.$('.player-playlist')
			.empty()
			.append(playlist.map(function(track, i) {
				var self = this;
				var $li = $(this.playlistTemplate(track));
				$li.find('.do-remove').on('click', function() {
					// Index +1 to exclude the current track.
					self.model.removeFromPlaylist(i + 1);
				});
				return $li;
			}, this));
	},

	doToggleState: function() {
		this.model.set('state', this.model.get('state') !== 'playing' ? 'playing' : 'paused');
	},

	doNext: function() {
		this.model.next();
	},

	doToggleVolume: function() {
		var vol = this.model.get('volume');
		if (vol !== 0) {
			this.oldVolume = vol;
		}
		this.model.set('volume', vol === 0 ? this.oldVolume || 0.01 : 0);
	},

	doSetProgress: function() {
		this.model.set('progress', parseInt(this.$('.do-set-progress').val(), 10));
	},

	doSetVolume: function() {
		var $input = this.$('.do-set-volume');
		var vol = parseInt($input.val(), 10) / parseInt($input.attr('max'), 10);
		this.model.set('volume', vol);
	},

	template: _.template(
		'<div class="player-now-playing">'+
			'<div class="track-art"></div>'+
			'<p class="track-album"></p>'+
			'<p class="track-title"></p>'+
			'<p class="track-artist"></p>'+
			'<div class="input-group">'+
				'<p class="input-group-addon track-duration">'+
					'<span class="current"></span> / <span class="total"></span>'+
				'</p>'+
				'<input class="do-set-progress" type="range" min="0" max="100" />'+
			'</div>'+
		'</div>'+

		'<div class="player-controls">'+
			'<div class="input-group">'+
				'<span class="input-group-btn">'+
					'<button class="btn btn-default glyphicon glyphicon-play do-toggle-state"></button>'+
				'</span>'+
				'<span class="input-group-btn">'+
					'<button class="btn btn-default glyphicon glyphicon-forward do-next"></button>'+
				'</span>'+
				'<span class="input-group-btn">'+
					'<button class="btn btn-default glyphicon glyphicon-volume-off do-toggle-volume"></button>'+
				'</span>'+
				'<input class="do-set-volume" type="range" min="0" max="100" value="0" />'+
			'</div>'+
		'</div>'+

		'<ul class="player-playlist"></ul>'
	),
	playlistTemplate: _.template(
		'<li class="queuedby-<%= queuedby %>">'+
			'<button class="do-remove glyphicon glyphicon-remove"></button>'+
			'<span class="track-artist"><%- artist %></span> - <span class="track-name"><%- title %></span>'+
		'</li>'
	),

});
