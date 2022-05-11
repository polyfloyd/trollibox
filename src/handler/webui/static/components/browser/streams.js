app.component('browser-streams', {
	mixins: [ApiMixin, PlaylistMixin],
	data: function() {
		return {
			streams: [],
			editStream: null,
			editError: '',
		};
	},
	template: `
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
	`,
	created: function() {
		this._ev = new EventSource(`${this.urlroot}data/streams/events`);
		this._ev.addEventListener('streams', async event => {
			this.streams = JSON.parse(event.data).streams
				.map(stream => { return {...stream, uri: stream.url}; })
				.sort((a, b) => stringCompareCaseInsensitive(a.title, b.title));
		});
	},
	destroyed: function() {
		this._ev.close();
	},
	methods: {
		removeStream: async function(stream) {
			let url = `${this.urlroot}data/streams?filename=${encodeURIComponent(stream.filename)}`;
			let response = await fetch(url, {method: 'DELETE'});
			if (!response.ok) {
				throw new Error('Unable to remove stream');
			}
		},
		addStream: async function(stream) {
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
});
