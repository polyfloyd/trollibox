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
				:class="currentTrack ? ['queuedby-'+currentTrack.queuedby] : []">
				<track-art :urlroot="urlroot" :selected-player="selectedPlayer" :track="currentTrack" />
				<p class="track-album">{{ currentTrack && currentTrack.album }}</p>
				<p class="track-title">{{ currentTrack && currentTrack.title }}</p>
				<p class="track-artist">{{ currentTrack && currentTrack.artist }}</p>

				<div class="track-time">
					<span v-if="currentTrack">{{ durationToString(time) }}</span>
					<span v-else>0:00</span>
					<input type="range"
						min="0" :max="currentTrack ? currentTrack.duration : 100"
						:value="time"
						title="Seek in the current track"
						@click="setTime($event.target.value|0)" />
					<span v-if="currentTrack && currentTrack.duration">
						{{ durationToString(currentTrack.duration) }}
					</span>
					<span v-else>∞</span>
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
		this.ev.addEventListener('playlist', async event => {
			let { index, tracks, time } = JSON.parse(event.data);
			this.index = index;
			this.playlist = tracks;
			this.time = time;
		});
		this.ev.addEventListener('state', event => {
			this.state = JSON.parse(event.data).state;
		});
		this.ev.addEventListener('time', event => {
			this.time = JSON.parse(event.data).time;
		});
		this.ev.addEventListener('volume', event => {
			this.volume = JSON.parse(event.data).volume / 100;
		});
		this.ev.addEventListener('library', async event => {
			await this.reloadTrackLibrary();
		});
		this.reloadTrackLibrary();
	},
	destroyed: function() {
		this._ev.close();
	},
	mounted: function() {
		document.body.addEventListener('keypress', this.onKey);
	},
	beforeDestroy: function() {
		document.body.removeEventListener('keypress', this.onKey);
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
			if (window.innerWidth >= 992) {
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

		reloadTrackLibrary: async function() {
			let res = await fetch(`${this.urlroot}data/player/${this.selectedPlayer}/tracks`);
			if (res.status >= 400) {
				throw new Error('could not fetch tracks');
			}
			let { tracks } = await res.json();
			this.$emit('update:tracks', tracks);
		},

		onKey: function(event) {
			if (event.target != document.body) return;
			switch (event.key) {
			case ' ':
				this.setState(this.state != 'playing' ? 'playing' : 'paused');
				break;
			case '<':
				this.setIndex(-1, true);
				break;
			case '>':
				this.setIndex(1, true);
				break;
			case 'ArrowUp':
				this.setVolume(this.volume + 0.05);
				break;
			case 'ArrowDown':
				this.setVolume(this.volume - 0.05);
				break;
			case 'ArrowLeft':
				this.setTime(this.time - 5);
				break;
			case 'ArrowRight':
				this.setTime(this.time + 5);
				break;
			case 'c':
				if (confirm('Are you sure you would like to clear the playlist?')) {
					this.clearPlaylist();
				}
				break;
			}
		},
	},
});
