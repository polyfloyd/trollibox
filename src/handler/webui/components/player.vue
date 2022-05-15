<template>
	<div class="player" :class="['player-'+state]">
		<draggable class="player-playlist player-past"
			v-model="pastPlaylist" item-key="key" group="playlist" @end="dragEnd">
			<template #item="{element, index}">
				<player-playlist-track v-bind="element" @remove="removeFromPlaylist(index)" />
			</template>
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
					:disabled="!currentTrack || !currentTrack.duration"
					title="Seek in the current track"
					@change="setTime($event.target.value|0)" />
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
					:disabled="state == 'stopped'"
					@change="setVolume($event.target.value/100)" />
				<span class="glyphicon glyphicon-volume-up"></span>
			</div>

			<div v-if="connectionState == 'connected'" class="player-controls">
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
			<div v-else class="player-controls">
				<button class="btn btn-default glyphicon glyphicon-refresh" title="Reconnect"
					:disabled="connectionState == 'connecting'" @click="reconnect"></button>
			</div>
		</div>

		<draggable class="player-playlist player-future"
			v-model="futurePlaylist" item-key="key" group="playlist" @end="dragEnd">
			<template #item="{element, index}">
				<player-playlist-track v-bind="element" @remove="removeFromPlaylist(currentIndex + index + 1)" />
			</template>
		</draggable>
	</div>
</template>

<script>
	import ApiMixin from './mixins/api.js';
	import TrackArt from './track-art.vue';
  import Draggable from 'vuedraggable';
  import PlayerMixin from './mixins/player.js';
  import PlayerPlaylistTrack from './player-playlist-track.vue';
  import TrackMixin from './mixins/track.js';

	export default {
		mixins: [ApiMixin, TrackMixin, PlayerMixin],
		components: {
			Draggable,
			TrackArt,
			PlayerPlaylistTrack,
		},
		emits: ['update:tracks'],
		data: function() {
			return {
				playlist: [],
				currentIndex: -1,
				state: 'stopped',
				time: 0,
				volume: 0,
				connectionState: 'disconnected',
			};
		},
		created: function() {
			this.reconnect();
		},
		destroyed: function() {
			this.ev.close();
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
					if (this.currentIndex == -1) return [];
					return this.playlist.slice(0, this.currentIndex);
				},
				set: function() {}, // Implemented by dragEnd.
			},
			futurePlaylist: {
				get: function() {
					if (this.currentIndex == -1) return this.playlist;
					return this.playlist.slice(this.currentIndex + 1);
				},
				set: function() {}, // Implemented by dragEnd.
			},
			currentTrack: function() {
				if (this.currentIndex == -1) return null;
				return this.playlist[this.currentIndex];
			},
		},
		watch: {
			playlist: function() {
				// Scroll to the current track so it is aligned to the top of the
				// container. But only if the player is the initialy selected view.
				if (window.innerWidth >= 992 && this.$el) {
					this.$el.querySelector('.player-current').scrollIntoView();
				}
			},
			state: function() { this.reloadProgressUpdater(); },
			time: function() { this.reloadProgressUpdater(); },
			currentIndex: function() { this.reloadProgressUpdater(); },
		},
		methods: {
			reloadProgressUpdater: function() {
				clearInterval(this.timeUpdater);
				if (this.currentIndex != -1 && this.state === 'playing') {
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
			reconnect: function() {
				if (this.ev) this.ev.close();
				this.ev = new EventSource(`${this.urlroot}data/player/${this.selectedPlayer}/events`);
				this.ev.addEventListener('error', () => { this.connectionState = 'disconnected'; });
				this.ev.addEventListener('open', () => { this.connectionState = 'connected'; });
				this.connectionState = 'connecting';
				this.ev.addEventListener('playlist', async event => {
					let { index, tracks, time } = JSON.parse(event.data);
					this.currentIndex = index;
					this.playlist = tracks.map(track => {
						return {key: `${track.uri}__${track.index}`, ...track};
					});
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
				event.preventDefault();
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
				case '+':
					this.setVolume(this.volume + 0.05);
					break;
				case '-':
					this.setVolume(this.volume - 0.05);
					break;
				case 'b':
					this.setTime(this.time - 5);
					break;
				case 'f':
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
	}
</script>

<style>
	.player {
		padding: 15px;
		color: var(--color-text);
		background-color: var(--color-bg-elem);
	}

	.player-current {
		font-size: 1.2em;
		padding: 15px 0;
	}

	.player-current .track-album {
		text-align: center;
		word-wrap: break-word;
	}

	.player-current .track-title {
		margin-bottom: 0;
		font-size: 1.3em;
		word-wrap: break-word;
	}

	.player-current .track-artist {
		font-size: 0.8em;
	}

	.player-current .track-art {
		width: 270px;
		height: 270px;
		margin: 0 auto 5px auto;
	}

	.player-current.queuedby-user .track-title:after {
		content: "*";
		margin-left: 0.3em;
		color: var(--color-accent);
	}

	.player .track-artist:empty:before,
	.player .track-title:empty:before {
		content: "None";
		opacity: 0.7;
	}

	.player .input-group-addon {
		padding: 0;
		border: none;
		font-size: inherit;
		color: inherit;
		background: none;
	}

	.player .track-time {
		margin-bottom: 0.3em;
		display: flex;
		font-size: 0.8em;
	}

	.player.player-stopped .track-time > * {
		color: var(--color-text-inactive);
	}

	.player .track-time > * {
		font-size: 0.8em;
	}

	.player .track-time > span {
		width: 46px;
		flex-shrink: 0;
		text-align: center;
	}

	.player-current .track-time > input {
		width: calc(100% - 0.3em);
		margin: 0.3em 0;
		height: 0.7em;
	}

	.player-volume {
		margin-bottom: 0.3em;
		display: flex;
	}

	.player-volume > span {
		width: 46px;
		text-align: center;
		flex-shrink: 0;
	}

	.player-controls {
		display: flex;
	}

	.player-controls > * {
		flex-grow: 1;
	}

	.player.player-playing .do-play,
	.player.player-paused .do-pause {
		display: none;
	}

	.player-playlist {
		margin: 0;
		padding-left: 0;
		font-size: 1.1em;
		list-style-type: none;
	}

	.player-playlist li {
		cursor: move;
	}

	.player-playlist li:not(:last-child) {
		border-bottom: 2px solid var(--color-bg);
	}

	.player-playlist li .track-artist:empty:before {
		content: "";
	}

	.player-playlist li .track-artist:not(:empty):after {
		content: " - ";
	}

	.player-playlist li.sortable-placeholder {
		height: 21px;
	}

	.player-playlist li .do-remove {
		padding: 0 0.4em 0 0;
		border: none;
		color: var(--color-accent);
		background: none;
	}

	.player-playlist li.queuedby-user:after {
		content: "*";
		margin-left: 0.3em;
		color: var(--color-accent);
	}

	.player-playlist.player-past li span {
		color: var(--color-text-inactive);
	}

	.player-playlist.player-future {
		min-height: calc(100% + 16px);
	}
</style>
