app.component('browser-files', {
	mixins: [ApiMixin, TrackMixin, PlaylistMixin],
	props: {
		library: {required: true, type: Array},
	},
	data: function() {
		return {path: '/'};
	},
	template: `
		<div class="browser-files view tab-view">
			<div v-for="node in shownDirs" class="tab">
				<div class="browser-files-header">
					<a v-if="path != '/'" class="glyphicon glyphicon-arrow-left do-pop-tab" @click="pop()"></a>
					<h2 @click="appendNodeToPlaylist(node, $event)">{{ node.name }}/</h2>
				</div>
				<ul class="result-list">
					<template v-for="file in node.files">
						<li v-if="!file.track"
							class="type-dir" :class="{active: (path+'/').indexOf(file.path+'/') == 0}"
							@click="path = file.path">{{ file.name }}/</li>
					</template>
					<template v-for="file in node.files">
						<li v-if="file.track"
							class="type-track"
							:title="formatTrackTitle(file.track)"
							@click="appendNodeToPlaylist(file, $event)">
							<span class="track-artist">{{ file.track.artist }}</span>
							<span class="track-title">{{ file.track.title }}</span>
							<span class="track-duration">{{ durationToString(file.track.duration) }}</span>
							<span class="track-album">{{ file.track.album }}</span>
							<span class="glyphicon glyphicon-plus"></span>
						</li>
					</template>
				</ul>
			</div>
		</div>
	`,
	computed: {
		commonPath: function() {
			if (this.library.length == 0) return '';
			return this.library.reduce((commonPath, track) => {
				for (let i = 0; i < commonPath.length; i++) {
					if (track.uri[i] != commonPath[i]) {
						return commonPath.substring(0, i);
					}
				}
				return commonPath;
			}, this.library[0].uri);
		},
		tree: function() {
			// This property has been moved out of loop body so we can allow
			// the browser to make the loop tighter.
			let commonPath = this.commonPath;

			return this.library.reduce((tree, track) => {
				let idPath = this.trimSlashes(track.uri.substring(commonPath.length));
				idPath.split('/').reduce((prev, pathPart, i, parts) => {
					let path = '/'+this.join([prev[0], pathPart]);
					let dir  = prev[1].files;

					if (i === parts.length - 1) {
						dir[pathPart] = {name: pathPart, path, track};
						return; // Last iteration.
					}

					return [
						path,
						dir[pathPart] || (dir[pathPart] = {name: pathPart, path, files: {}}),
					];
				}, [ '', tree ]);

				return tree;
			}, { path: '/', files: {} });
		},
		shownDirs: function() {
			if (this.path == '/') return [this.tree];
			let pathParts = ['/'].concat(this.path.split('/').slice(1));
			return pathParts
				.map((part, i) => {
					if (i == 0) return '/';
					return this.join(pathParts.slice(0, i + 1));
				})
				.map((path) => this.nodeByPath(path));
		},
	},
	methods: {
		pop: function() {
			if (this.path == '/') return;
			this.path = this.path.replace(/\/[^\/]+$/, '');
		},
		nodeByPath: function(path) {
			if (path === '/') return this.tree;
			let node = this.trimSlashes(path).split('/')
				.reduce((node, pathPart) => node ? node.files[pathPart] : null, this.tree);
			if (!node) throw new Error(`no such node: ${path}`);
			return node;
		},
		appendNodeToPlaylist: function(node, event) {
			if (node.track) {
				this.appendToPlaylist(node.track, event);
				return;
			}

			let tracks = this.library.filter((track) => {
				return track.uri.substring(this.commonPath.length).indexOf(node.path.slice(1)) === 0;
			});
			if (tracks.length > 20 && !confirm(`You are about to add ${tracks.length} tracks to the playlist. Is that okay?`)) {
				return;
			}
			this.appendToPlaylist(tracks, event);
		},

		join: function(parts) {
			return this.trimSlashes(Array.prototype.join.call(parts, '/'));
		},
		trimSlashes: function(path) {
			if (path[0] == '/') path = path.substring(1);
			if (path[path.length - 1] == '/') path = path.substring(0, path.length - 1);
			return path
		},
	},
});
