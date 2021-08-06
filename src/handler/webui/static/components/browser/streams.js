Vue.component('browser-streams', {
	mixins: [ApiMixin, PlaylistMixin],
	data: function() {
		return {
			streams: [],
			editStream: null,
		};
	},
	template: `
		<div class="browser-streams">
			<h2>
				Network Streams
				<span class="glyphicon glyphicon-plus do-add-stream"
					@click="showEditStreamDialog(null)"></span>
			</h2>
			<ul class="result-list grid-list">
				<li v-for="stream in streams" :title="stream.title"
					@click="appendToPlaylist(stream, $event.target)">
					<img class="ratio" src="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAABmJLR0QA/wD/AP+gvaeTAAAACXBIWXMAAAsTAAALEwEAmpwYAAAADUlEQVQI12NgYGBgAAAABQABXvMqOgAAAABJRU5ErkJggg==" />
					<track-art :urlroot="urlroot" :selectedPlayer="selectedPlayer" :track="stream" />
					<span class="stream-title">{{ stream.title }}</span>
					<span class="glyphicon glyphicon-plus do-add"></span>
					<button class="glyphicon glyphicon-remove do-remove"
						@click.stop="removeStream(stream)"></button>
					<button class="glyphicon glyphicon-edit do-edit"
						@click.stop="showEditStreamDialog(stream)"></button>
				</li>
			</ul>

			<div v-if="editStream" class="modal fade show" style="opacity: 1">
				<div class="modal-dialog">
					<form class="modal-content dialog-add-stream">
						<div class="modal-header">
							<button type="button" class="close" data-dismiss="modal" aria-label="Close" @click="editStream = null"><span aria-hidden="true">&times;</span></button>
							<h4 class="modal-title">Add Stream</h4>
						</div>
						<div class="modal-body">
							<div class="input-group">
								<input class="form-control" type="text" v-model="editStream.url"
									placeholder="URL" required />
								<input class="form-control" type="text" v-model="editStream.title"
									placeholder="Title" required />
								<input class="form-control" type="text" v-model="editStream.arturi"
									:placeholder="editStream.hasart ? 'Keep current image URL' : 'Image URL'" />
							</div>
							<track-art v-if="editStream.hasart"
								:urlroot="urlroot"
								:selected-player="selectedPlayer"
								:track="editStream"
								class="art-preview" />
							<img v-else-if="editStream.arturi"
								:src="editStream.arturi"
								class="art-preview"/>
						</div>
						<div class="modal-footer">
							<button type="button" class="btn btn-default" data-dismiss="modal" @click="editStream = null">Cancel</button>
							<input type="submit" class="btn btn-default do-add" value="Add" @click="addStream(editStream); editStream = null" />
						</div>
					</form>
				</div>
			</div>
		</div>
	`,
	created: function() {
		this._ev = new EventSource(`${this.urlroot}data/streams/events`);
		this._ev.onopen = () => {
			// Reload all state to ensure that we are in sync.
			this.reload()
				.catch(err => console.error(err));
		};
		this._ev.addEventListener('library:tracks', async event => {
			this.reload()
				.catch(err => console.error(err));
		});
		this.reload()
			.catch(err => console.error(err));
	},
	destroyed: function() {
		this._ev.close();
	},
	methods: {
		showEditStreamDialog: function(stream) {
			this.editStream = stream || {};
		},
		reload: async function() {
			this.streams = await this.loadStreams();
		},

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
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ stream }),
			});
			if (!response.ok) {
				throw new Error('Unable to add stream');
			}
		},
		loadStreams: async function() {
			let response = await fetch(`${this.urlroot}data/streams`);
			if (!response.ok) {
				throw new Error('Unable to list streams');
			}
			let { streams } = await response.json();
			return streams
				.map(stream => { return {...stream, uri: stream.url}; })
				.sort((a, b) => stringCompareCaseInsensitive(a.title, b.title));
		},
	},
});
