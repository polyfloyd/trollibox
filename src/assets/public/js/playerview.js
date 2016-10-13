'use strict';

var PlayerView = Backbone.View.extend({
	tagName:   'div',
	className: 'player',

	events: {
		'click .do-previous':      'doPrevious',
		'click .do-next':          'doNext',
		'click .do-clear':         'doClear',
		'click .do-toggle-state':  'doToggleState',
		'input .do-set-volume':    'doSetVolume',
		'input .do-set-time':      'doSetProgress',
		'dragover':                'doMakeDroppable',
		'dragenter':               'doMakeDroppable',
		'drop':                    'doAcceptRawFiles',
	},

	initialize: function() {
		this.listenTo(this.model, 'change:current',  this.renderCurrent);
		this.listenTo(this.model, 'change:current',  this.renderPlaylist);
		this.listenTo(this.model, 'change:current',  this.renderProgress);
		this.listenTo(this.model, 'change:playlist', this.renderPlaylist);
		this.listenTo(this.model, 'change:time',     this.renderProgress);
		this.listenTo(this.model, 'change:state',    this.renderState);
		this.listenTo(this.model, 'change:volume',   this.renderVolume);
		this.render();
	},

	render: function() {
		var self = this;

		this.$el.html(this.template());
		this.renderCurrent();
		this.renderPlaylist();
		this.renderProgress();
		this.renderState();
		this.renderVolume();

		var sortable = this.$('.player-playlist').sortable({
			forcePlaceholderSize: true,
			items:                'li',
		});
		sortable.bind('sortupdate', function(event, update) {
			self.doReorderPlaylist(event, update);
		});
	},

	renderCurrent: function() {
		var cur = this.model.getCurrentTrack() || {};

		showTrackArt(this.$('.track-art'), this.model, cur);
		this.$('.player-current .track-album').text(cur.album || '');
		this.$('.player-current .track-artist').text(cur.artist || '');
		this.$('.player-current .track-title').text(cur.title || '');
		this.$('.player-current')
			.removeClass('queuedby-system queuedby-user')
			.addClass('queuedby-'+cur.queuedby)
			.toggleClass('track-infinite', cur.duration == 0);
		this.$('.track-duration-total')
			.text(cur.duration ? durationToString(cur.duration) : '');
		this.$('.do-set-time')
			.attr('max', cur.duration || 0);
	},

	renderProgress: function() {
		var pr = this.model.get('time') || 0;
		var text = this.model.getCurrentTrack() ? durationToString(pr) : '';
		this.$('.track-duration-current').text(text);
		this.$('.do-set-time').val(pr);
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
		var vol = this.model.get('volume');
		var $setVol = this.$('.do-set-volume');
		$setVol.val(vol * parseInt($setVol.attr('max'), 10));
	},

	renderPlaylist: function() {
		var playlist = this.model.get('playlist');
		if (playlist.length > 0) {
			// Slice off the history and currently playing track.
			playlist = playlist.slice(this.model.get('current') + 1);
		}

		var $pl = this.$('.player-playlist');
		$pl.empty();
		$pl.append(playlist.map(function(track, i) {
			var $li = $(this.playlistTemplate(track));
			$li.on('click', function() {
				if (Hotkeys.state.ctrl) {
					var cur = this.model.get('current');
					this.model.moveInPlaylist(cur + 1 + i, cur + 1);
				}
			}.bind(this));
			$li.find('.do-remove').on('click', function(event) {
				event.preventDefault();
				this.model.removeFromPlaylist(this.model.get('current') + i + 1);
			}.bind(this));
			return $li;
		}, this));
		$pl.sortable('reload');
	},

	doToggleState: function() {
		this.model.set('state', this.model.get('state') !== 'playing' ? 'playing' : 'paused');
	},

	doPrevious: function() {
		this.model.setCurrent(-1, true);
	},

	doNext: function() {
		this.model.setCurrent(1, true);
	},

	doClear: function() {
		var pl = this.model.get('playlist');
		if (pl.length > this.model.get('current')+1) {
			var rem = [];
			for (var i = this.model.get('current')+1; i < pl.length; i++) {
				rem.push(i);
			}
			this.model.removeFromPlaylist(rem);
		}
	},

	doSetProgress: function() {
		this.model.set('time', parseInt(this.$('.do-set-time').val(), 10));
	},

	doSetVolume: function() {
		var $input = this.$('.do-set-volume');
		var vol = parseInt($input.val(), 10) / parseInt($input.attr('max'), 10);
		this.model.set('volume', vol);
	},

	doReorderPlaylist: function(event, update) {
		var pl = this.model.get('playlist');
		var ci = this.model.get('current');
		this.model.moveInPlaylist(update.oldindex + ci + 1, update.item.index() + ci + 1);
	},

	doMakeDroppable: function(event) {
		event.preventDefault();
		return false;
	},

	doAcceptRawFiles: function(event) {
		event.preventDefault();
		this.model.playRawTracks(event.originalEvent.dataTransfer.files);
		return false;
	},

	template: _.template(
		'<div class="player-current">'+
			'<div class="track-art"></div>'+
			'<p class="track-album"></p>'+
			'<p class="track-title"></p>'+
			'<p class="track-artist"></p>'+

			'<div class="input-group">'+
				'<p class="input-group-addon">'+
					'<span class="track-duration-current"></span>'+
					'<span class="track-duration-total"></span>'+
				'</p>'+
				'<input class="do-set-time" type="range" min="0" max="100" title="Seek in the current track" />'+
			'</div>'+
			'<div class="input-group">'+
				'<span class="input-group-btn">'+
					'<button class="btn btn-default glyphicon glyphicon-step-backward do-previous" title="Go back to the previous track"></button>'+
				'</span>'+
				'<span class="input-group-btn">'+
					'<button class="btn btn-default glyphicon glyphicon-play do-toggle-state" title="Pause/play"></button>'+
				'</span>'+
				'<span class="input-group-btn">'+
					'<button class="btn btn-default glyphicon glyphicon-step-forward do-next" title="Skip to the next track"></button>'+
				'</span>'+
				'<span class="input-group-btn">'+
					'<button class="btn btn-default glyphicon glyphicon-ban-circle do-clear" title="Clear the playlist"></button>'+
				'</span>'+
				'<input class="do-set-volume" type="range" min="0" max="100" value="0" title="Set volume level" />'+
			'</div>'+
		'</div>'+

		'<ul class="player-playlist"></ul>'
	),
	playlistTemplate: _.template(
		'<li class="queuedby-<%= queuedby %>">'+
			'<button class="do-remove glyphicon glyphicon-remove"></button>'+
			'<span class="track-artist"><%- artist %></span><span class="track-title"><%- title %></span>'+
		'</li>'
	),

});
