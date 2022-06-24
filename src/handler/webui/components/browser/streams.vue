<template>
	<div class="browser-streams">
		<div class="stream-header">
			<h2>
				Network Streams
				<span v-if="!editStream" class="glyphicon glyphicon-plus do-add-stream"
					@click="editStream = {}"></span>
			</h2>
			<div v-if="editStream" class="edit-stream">
				<img v-if="editStream.arturi" class="art-preview" :src="editStream.arturi" />
				<track-art v-else-if="editStream"
					class="art-preview"
					:urlroot="urlroot"
					:selected-player="selectedPlayer"
					:track="editStream" />
				<div class="input-group">
					<input class="form-control" type="text" v-model="editStream.url"
						placeholder="URL" required />
					<input class="form-control" type="text" v-model="editStream.title"
						placeholder="Title" required />
					<input class="form-control" type="text" v-model="editStream.arturi"
						placeholder="Image URL" />
				</div>
				<div class="input-group">
					<button class="btn btn-default" @click="editStream = null">Cancel</button>
					<button class="btn btn-default" @click="addStream(editStream); ">Save</button>
				</div>
				<div v-if="editError" class="input-group error-message">{{ editError }}</div>
			</div>
		</div>
		<div class="stream-list">
			<div class="grid-list">
				<div v-for="stream in streams" class="grid-item" :title="stream.title"
					@click="appendToPlaylist(stream, $event)">
					<track-art :urlroot="urlroot" :selectedPlayer="selectedPlayer" :track="stream" />
					<span class="stream-title">{{ stream.title }}</span>
						<span class="glyphicon glyphicon-plus do-add"></span>
					<button class="glyphicon glyphicon-remove do-remove"
						@click.stop="removeStream(stream)"></button>
					<button class="glyphicon glyphicon-edit do-edit"
						@click.stop="editStream = stream"></button>
				</div>
			</div>
		</div>
	</div>
</template>

<script>
	import ApiMixin from '../mixins/api.js';
	import PlaylistMixin from '../mixins/playlist.js';
	import TrackArt from '../track-art.vue';
	import { stringCompareCaseInsensitive } from '../mixins/util.js';

	export default {
		mixins: [ApiMixin, PlaylistMixin],
		components: {
			TrackArt,
		},
		data() {
			return {
				streams: [],
				editStream: null,
				editError: '',
			};
		},
		mounted() {
			this._ev = new EventSource(`${this.urlroot}data/streams/events`);
			this._ev.addEventListener('streams', async event => {
				this.streams = JSON.parse(event.data).streams
					.map(stream => { return {...stream, uri: stream.url}; })
					.sort((a, b) => stringCompareCaseInsensitive(a.title, b.title));
			});
		},
		unmounted() {
			this._ev.close();
		},
		methods: {
			async removeStream(stream) {
				let url = `${this.urlroot}data/streams?filename=${encodeURIComponent(stream.filename)}`;
				let response = await fetch(url, {method: 'DELETE'});
				if (!response.ok) {
					throw new Error('Unable to remove stream');
				}
			},
			async addStream(stream) {
				let response = await fetch(`${this.urlroot}data/streams`, {
					method: 'POST',
					headers: {'Content-Type': 'application/json'},
					body: JSON.stringify({stream}),
				});
				if (!response.ok) {
					let body = await response.json();
					this.editError = body.error;
					return;
				}

				this.editStream = null;
				this.editError = '';
			},
		},
	}
</script>

<style>
.browser-streams {
	width: 100%;
	display: flex;
	flex-flow: column;
}

.browser-streams .do-add-stream {
	display: inline-block;
	font-size: 0.6em;
	vertical-align: middle;
	color: var(--color-accent);
	cursor: pointer;
}

.browser-streams .stream-header {
	margin-bottom: 15px;
}

.browser-streams .edit-stream .art-preview {
	width: 160px;
	height: 160px;
	margin-left: 15px;
	float: right;
}

.browser-streams .input-group + .input-group {
	margin-top: 15px;
}

.browser-streams .stream-list {
	width: 100%;
	height: 100%;
	overflow-y: scroll;
}

.browser-streams .grid-item {
	cursor: pointer;
}

.browser-streams .grid-item > * {
	position: absolute;
	font-size: 1.2em;
	background-color: var(--color-bg);
}

.browser-streams .grid-item > .stream-title {
	padding: 0 0.2em;
	position: absolute;
	top: 0;
	left: 0;
	right: 0;
	text-overflow: ellipsis;
	overflow: hidden;
}

.browser-streams .grid-item:hover > .stream-title {
	text-overflow: initial;
	overflow: initial;
	word-break: break-word;
}

.browser-streams .grid-item > button {
	padding: 0.2em;
	top: auto;
	bottom: 0;
	border: none;
	opacity: 0;
	color: var(--color-accent);
}

.browser-streams .grid-item:hover > button {
	opacity: 1;
}

.browser-streams .grid-item > button.do-remove {
	right: 0;
}

.browser-streams .grid-item > button.do-edit {
	left: 0;
}

.browser-streams .grid-item > .do-add {
	right: 0;
	color: var(--color-accent);
	opacity: 0;
}

.browser-streams .grid-list > .grid-item:hover .do-add {
	opacity: 1;
}
</style>
