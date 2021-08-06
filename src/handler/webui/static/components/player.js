Vue.component('player-playlist-track', {
	props: {
		title: {required: true, type: String},
		artist: String,
		queuedby: {required: true, type: String},
	},
	template: `
		<li class="track-item" :class="['queuedby-'+queuedby]" >
			<button class="glyphicon glyphicon-remove do-remove" @click="$emit('remove')"></button>
			<span class="track-artist">{{ artist }}</span><span class="track-title">{{ title }}</span>
		</li>
	`,
});


Vue.component('player', {
	mixins: [ApiMixin, TrackMixin, PlayerMixin],
	data: function() {
		return {
			playlist: [],
			index: -1,
			state: 'stopped',
			time: 0,
			volume: 0,
		};
	},
	template: `
		<div class="player" :class="['player-'+state]">
			<draggable class="player-playlist player-past"
				v-model="pastPlaylist" group="playlist" @end="dragEnd">
				<player-playlist-track v-for="(track, i) in pastPlaylist" :key="i"
					v-bind="track"
					@remove="removeFromPlaylist(i)" />
			</draggable>

			<div class="player-current"
				:class="currentTrack ? [currentTrack.duration == 0  ? 'track-infinite' : '', 'queuedby-'+currentTrack.queuedby] : []">
				<track-art :urlroot="urlroot" :selected-player="selectedPlayer" :track="currentTrack" />
				<p class="track-album">{{ currentTrack && currentTrack.album }}</p>
				<p class="track-title">{{ currentTrack && currentTrack.title }}</p>
				<p class="track-artist">{{ currentTrack && currentTrack.artist }}</p>

				<div class="track-time">
					<span class="track-time-current">{{ currentTrack && durationToString(time) }}</span>
					<input type="range"
						min="0" :max="currentTrack ? currentTrack.duration : 100"
						:value="time"
						title="Seek in the current track"
						@click="setTime($event.target.value|0)" />
					<span class="track-time-total">{{ currentTrack && durationToString(currentTrack.duration) }}</span>
				</div>

				<div class="player-volume">
					<span class="glyphicon glyphicon-volume-down"></span>
					<input type="range"
						min="0" max="100" :value="volume * 100"
						title="Set volume level"
						@click="setVolume($event.target.value/100)" />
					<span class="glyphicon glyphicon-volume-up"></span>
				</div>

				<div class="player-controls">
					<button class="btn btn-default glyphicon glyphicon-step-backward" title="Go back to the previous track"
						@click="setIndex(-1, true)"></button>
					<button class="btn btn-default glyphicon"
						:class="[state == 'playing' ? 'glyphicon-pause' : 'glyphicon-play']"
						title="Pause/play"
						@click="setState(state != 'playing' ? 'playing' : 'paused')"></button>
					<button class="btn btn-default glyphicon glyphicon-step-forward" title="Skip to the next track"
						@click="setIndex(1, true)"></button>
					<button class="btn btn-default glyphicon glyphicon-ban-circle" title="Clear the playlist"
						@click="clearPlaylist()"></button>
				</div>
			</div>

			<draggable class="player-playlist player-future"
				v-model="futurePlaylist" group="playlist" @end="dragEnd">
				<player-playlist-track v-for="(track, i) in futurePlaylist" :key="i"
					v-bind="track"
					@remove="removeFromPlaylist(i + index + 1)" />
			</draggable>
		</div>
	`,
	created: function() {
		this.ev = new EventSource(`${this.urlroot}data/player/${this.selectedPlayer}/events`);
		this.ev.onopen = () => {
			// Reload all state to ensure that we are in sync.
			this.reload()
				.catch(err => console.error(err));
		};
		this.ev.addEventListener('playlist', async event => {
			let { index, playlist } = await this.loadPlaylist();
			this.index = index;
			this.playlist = playlist;
		});
		this.ev.addEventListener('playstate', event => {
			this.state = JSON.parse(event.data).state;
		});
		this.ev.addEventListener('time', event => {
			this.time = JSON.parse(event.data).time;
		});
		this.ev.addEventListener('volume', event => {
			this.volume = JSON.parse(event.data).volume;
		});
		this.ev.addEventListener('tracks', async event => {
			await this.reloadTrackLibrary();
		});
		this.reload()
			.catch(err => console.error(err));
	},
	destroyed: function() {
		this._ev.close();
	},
	computed: {
		pastPlaylist: {
			get: function() {
				if (this.index == -1) return [];
				return this.playlist.slice(0, this.index);
			},
			set: function() {}, // Implemented by dragEnd.
		},
		futurePlaylist: {
			get: function() {
				if (this.index == -1) return this.playlist;
				return this.playlist.slice(this.index + 1);
			},
			set: function() {}, // Implemented by dragEnd.
		},
		currentTrack: function() {
			if (this.index == -1) return null;
			return this.playlist[this.index];
		},
	},
	watch: {
		playlist: function() {
			// Scroll to the current track so it is aligned to the top of the
			// container. But only if the player is the initialy selected view.
			if ($(window).width() >= 992) {
				this.$el.querySelector('.player-current').scrollIntoView();
			}
		},
		state: function() { this.reloadProgressUpdater(); },
		time: function() { this.reloadProgressUpdater(); },
		index: function() { this.reloadProgressUpdater(); },
	},
	methods: {
		reloadProgressUpdater: function() {
			clearInterval(this.timeUpdater);
			if (this.index != -1 && this.state === 'playing') {
				this.timeUpdater = setInterval(() => { this.time += 1; }, 1000);
			}
		},
		dragEnd: function(event) {
			let fromFuture = event.from.classList.contains('player-future');
			let toFuture = event.to.classList.contains('player-future');
			let from = event.oldIndex + (fromFuture ? this.pastPlaylist.length+1 : 0);
			let to = event.newIndex + (toFuture ? this.pastPlaylist.length+1 : 0);
			this.moveInPlaylist(from, to);
		},

		reload: async function() {
			let [
				{ index, playlist },
				state,
				time,
				volume,
			] = await Promise.all([
				this.loadPlaylist(),
				this.loadState(),
				this.loadTime(),
				this.loadVolume(),
			]);
			this.index = index;
			this.playlist = playlist;
			this.state = state;
			this.time = time;
			this.volume = volume;
			await this.reloadTrackLibrary();
		},
		loadPlaylist: async function() {
			let res = await fetch(`${this.urlroot}data/player/${this.selectedPlayer}/playlist`);
			if (res.status >= 400) {
				throw new Error('could not fetch tracks');
			}
			let { current, tracks } = await res.json();
			return { index: current, playlist: tracks };
		},
		loadState: async function() {
			let res = await fetch(`${this.urlroot}data/player/${this.selectedPlayer}/playstate`);
			if (res.status >= 400) {
				throw new Error('could not fetch tracks');
			}
			let { playstate } = await res.json();
			return playstate;
		},
		loadTime: async function() {
			let res = await fetch(`${this.urlroot}data/player/${this.selectedPlayer}/time`);
			if (res.status >= 400) {
				throw new Error('could not fetch tracks');
			}
			let { time } = await res.json();
			return time;
		},
		loadVolume: async function() {
			let res = await fetch(`${this.urlroot}data/player/${this.selectedPlayer}/volume`);
			if (res.status >= 400) {
				throw new Error('could not fetch tracks');
			}
			let { volume } = await res.json();
			return volume;
		},
		reloadTrackLibrary: async function() {
			let res = await fetch(`${this.urlroot}data/player/${this.selectedPlayer}/tracks`);
			if (res.status >= 400) {
				throw new Error('could not fetch tracks');
			}
			let { tracks } = await res.json();
			this.$emit('update:tracks', tracks);
		},
	},
});
