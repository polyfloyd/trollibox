class PlayerView {
	constructor(player) {
		this.player = player;
		this.player.addEventListener('change:index', () => this.renderCurrent());
		this.player.addEventListener('change:index', () => this.renderPlaylist());
		this.player.addEventListener('change:index', () => this.renderProgress());
		this.player.addEventListener('change:playlist', () => this.renderPlaylist());
		this.player.addEventListener('change:time', () => this.renderProgress());
		this.player.addEventListener('change:state', () => this.renderState());
		this.player.addEventListener('change:volume',  () => this.renderVolume());
		this.render();
	}

	render() {
		this.$el = $(playerViewTemplate());
		this.renderCurrent();
		this.renderPlaylist();
		this.renderProgress();
		this.renderState();
		this.renderVolume();

		this.$el.find('.do-previous').on('click', () => this.doPrevious());
		this.$el.find('.do-next').on('click', () => this.doNext());
		this.$el.find('.do-toggle-state').on('click', () => this.doToggleState());
		this.$el.find('.do-clear').on('click', () => this.doClear());
		this.$el.find('.do-add-netmedia').on('click', () => this.showNetmediaDialog());
		this.$el.find('.do-set-volume').on('click', () => this.doSetVolume());
		this.$el.find('.do-set-time').on('click', () => this.doSetProgress());
		this.$el.on('dragover', () => this.doMakeDroppable());
		this.$el.on('dragenter', () => this.doMakeDroppable());
		this.$el.on('drop', () => this.doAcceptRawFiles());

		const sortables = window.sortable(this.$el.find('.player-playlist'), {
			forcePlaceholderSize: true,
			items:                'li',
			acceptFrom:           '.player-playlist',
		});
		sortables.forEach(s => {
			s.addEventListener('sortupdate', event => {
				this.doReorderPlaylist(event);
			});
		});
	}

	renderCurrent() {
		const cur = this.player.getCurrentTrack() || {};

		showTrackArt(this.$el.find('.track-art'), this.player, cur);
		this.$el.find('.player-current .track-album').text(cur.album || '');
		this.$el.find('.player-current .track-artist').text(cur.artist || '');
		this.$el.find('.player-current .track-title').text(cur.title || '');
		this.$el.find('.player-current')
			.removeClass('queuedby-system queuedby-user')
			.addClass(`queuedby-${cur.queuedby}`)
			.toggleClass('track-infinite', cur.duration == 0);
		this.$el.find('.track-time-total')
			.text(cur.duration ? durationToString(cur.duration) : '');
		this.$el.find('.do-set-time').attr('max', cur.duration || 0);
	}

	renderProgress() {
		const pr = this.player.time || 0;
		const text = this.player.getCurrentTrack() ? durationToString(pr) : '';
		this.$el.find('.track-time-current').text(text);
		this.$el.find('.do-set-time').val(pr);
	}

	renderState() {
		this.$el.toggleClass('player-paused',  this.player.state === 'paused');
		this.$el.toggleClass('player-playing', this.player.state === 'playing');
		this.$el.toggleClass('player-stopped', this.player.state === 'stopped');

		this.$el.find('.do-toggle-state')
			.toggleClass('glyphicon-pause', this.player.state === 'playing')
			.toggleClass('glyphicon-play',  this.player.state !== 'playing');
	}

	renderVolume() {
		const $setVol = this.$el.find('.do-set-volume');
		$setVol.val(this.player.volume * parseInt($setVol.attr('max'), 10));
	}

	renderPlaylist() {
		[
			{
				$pl: this.$el.find('.player-playlist.player-past'),
				tracks: this.player.playlist.slice(0, this.player.index),
			},
			{
				$pl: this.$el.find('.player-playlist.player-future'),
				tracks: this.player.playlist.slice(this.player.index + 1),
			},
		].forEach((opt, l) => {
			opt.$pl.empty();
			opt.$pl.append(opt.tracks.map((track, i) => {
				const $li = $(playerViewPlaylistTemplate(track));
				const offset = l == 1 ? this.player.index + 1 : 0;
				$li.find('.do-remove').on('click', event => {
					event.preventDefault();
					this.player.removeFromPlaylist(offset + i);
				});
				$li.on('click', () => {
					if (Hotkeys.state.ctrl) {
						this.player.moveInPlaylist(offset + i, this.player.index);
					}
				});
				return $li;
			}));
			window.sortable(opt.$pl); // Reload sortable
		});

		// Scroll to the current track, but only if the player is the
		// initialy selected view.
		if ($(window).width() >= 992) {
			this.showCurrent();
		}
	}

	// Aligns the player to the top of the container.
	showCurrent() {
		this.$el.find('.player-current')[0].scrollIntoView();
	}

	doToggleState() {
		this.player.setState(this.player.state !== 'playing' ? 'playing' : 'paused');
	}

	doPrevious() {
		this.player.setIndex(-1, true);
	}

	doNext() {
		this.player.setIndex(1, true);
	}

	doClear() {
		if (this.player.playlist.length > this.player.index + 1) {
			let rem = [];
			for (let i = this.player.index+1; i < this.player.playlist.length; i++) {
				rem.push(i);
			}
			this.player.removeFromPlaylist(rem);
		}
	}

	showNetmediaDialog() {
		new AddMediaDialog({ model: this.player });
	}

	doSetProgress() {
		this.player.setTime(parseInt(this.$el.find('.do-set-time').val(), 10));
	}

	doSetVolume() {
		const $input = this.$el.find('.do-set-volume');
		const vol = parseInt($input.val(), 10) / parseInt($input.attr('max'), 10);
		this.player.setVolume(vol)
	}

	doReorderPlaylist(event) {
		const data = sortable(event.target, 'serialize');
		const offset = $parent => {
			return $parent.classList.contains('player-future')
				? this.player.index + 1
				: 0;
		};

		let from = offset(event.detail.origin.container) + event.detail.origin.index;
		let to = offset(event.detail.destination.container) + event.detail.destination.index;
		let fromPast = !event.detail.origin.container.classList.contains('player-future');
		let toFuture = event.detail.destination.container.classList.contains('player-future');
		if (fromPast && toFuture) {
			to -= 1;
		}
		this.player.moveInPlaylist(from, to);
	}

	doMakeDroppable(event) {
		event.preventDefault();
		return false;
	}

	doAcceptRawFiles(event) {
		event.preventDefault();
		this.player.playRawTracks(event.originalEvent.dataTransfer.files);
		return false;
	}
}

const playerViewTemplate = _.template(`
	<div class="player">
		<ul class="player-playlist player-past"></ul>

		<div class="player-current">
			<div class="track-art"></div>
			<p class="track-album"></p>
			<p class="track-title"></p>
			<p class="track-artist"></p>

			<div class="track-time">
				<span class="track-time-current"></span>
				<input class="do-set-time" type="range" min="0" max="100" title="Seek in the current track" />
				<span class="track-time-total"></span>
			</div>

			<div class="player-volume">
				<span class="glyphicon glyphicon-volume-down"></span>
				<input class="do-set-volume" type="range" min="0" max="100" value="0" title="Set volume level" />
				<span class="glyphicon glyphicon-volume-up"></span>
			</div>

			<div class="player-controls">
				<button class="btn btn-default glyphicon glyphicon-step-backward do-previous" title="Go back to the previous track"></button>
				<button class="btn btn-default glyphicon glyphicon-play do-toggle-state" title="Pause/play"></button>
				<button class="btn btn-default glyphicon glyphicon-step-forward do-next" title="Skip to the next track"></button>
				<button class="btn btn-default glyphicon glyphicon-ban-circle do-clear" title="Clear the playlist"></button>
				<button class="btn btn-default glyphicon glyphicon-cloud do-add-netmedia"></button>
			</div>
		</div>

		<ul class="player-playlist player-future"></ul>
	</div>
`);

const playerViewPlaylistTemplate = _.template(`
	<li class="queuedby-<%= queuedby %>">
		<button class="do-remove glyphicon glyphicon-remove"></button>
		<span class="track-artist"><%- artist %></span><span class="track-title"><%- title %></span>
	</li>
`);
