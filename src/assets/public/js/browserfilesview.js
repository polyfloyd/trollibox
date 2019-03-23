'use strict';

var BrowserFilesView = BrowserView.extend({
	tagName:   'div',
	className: 'view browser-files',

	initialize: function(options) {
		this.tabs = new TabView();
		this.$el.append(this.tabs.$el);
		this.model.addEventListener('change:tracks', this.updateTree.bind(this));
		this.updateTree();
	},

	determineCommonPath: function() {
		this.commonPath = '';
		if (this.model.tracks.length > 0) {
			this.commonPath = this.model.tracks.reduce(function(commonPath, track) {
				for (var i = 0; i < track.uri.length; i++) {
					if (track.uri[i] != commonPath[i]) {
						return commonPath.substring(0, i);
					}
				}
				return commonPath;
			}, this.model.tracks[0].uri);
		}
	},

	updateTree: function() {
		var self = this;
		this.determineCommonPath();
		this.tree = this.model.tracks.reduce(function(tree, track) {
			var idPath = track.uri.substring(self.commonPath.length);
			self.trimSlashes(idPath).split('/').reduce(function(prev, pathPart, i, parts) {
				var path = self.join(prev[0], pathPart);
				var dir  = prev[1].files;

				if (i === parts.length - 1) {
					dir[pathPart] = {
						track: track,
						name:  pathPart,
						path:  path,
					};
					return; // Last iteration.
				}

				return [
					path,
					dir[pathPart] || (dir[pathPart] = {
						path:  path,
						name:  pathPart,
						files: {},
					}),
				];
			}, [ '', tree ]);

			return tree;
		}, { path: '/', files: {} });

		this.render();
	},

	render: function() {
		this.tabs.clearTabs();
		try {
			this.showDirectory(this.getState());
		} catch (err) {
			this.showDirectory('/');
		}
	},

	setState: function(state) {
		this.shownDirectory = this.trimSlashes(state || '');
		try {
			this.showDirectory(this.shownDirectory);
		} catch (err) {
			this.showDirectory('/');
		}
		this.trigger('change-state', this.shownDirectory);
	},

	getState: function() {
		return this.trimSlashes(this.shownDirectory || '');
	},

	showDirectory: function(path, lastOnly) {
		var self = this;

		var trimmedPath = this.trimSlashes(path);
		var pathParts = trimmedPath.split('/');

		var shownDirs = [ this.getFile('/') ].concat(pathParts.reduce(function(prev, pathPart) {
			var path = self.join(prev[0], pathPart);
			return pathPart === '' ? prev : [
				path,
				prev[1].concat(self.getFile(path)),
			];
		}, [ '', [] ])[1]);

		var pathNotExist = shownDirs.some(function(shownDirs) {
			return !shownDirs;
		});
		if (pathNotExist) {
			throw new Error('The directory path "'+trimmedPath+'" does not exist');
		}

		shownDirs.forEach(function(dir, i) {
			if (lastOnly && i !== shownDirs.length - 1) {
				return;
			}

			var dirsAndFiles = Object.keys(dir.files).reduce(function(dirsAndFiles, filename) {
				var file = dir.files[filename];
				dirsAndFiles[!!file.track|0].push(file);
				return dirsAndFiles;
			}, [ [], [] ]).map(function(arr) {
				return arr.sort(function(a, b) {
					return stringCompareCaseInsensitive(a.name, b.name);
				});
			});
			var dirs   = dirsAndFiles[0];
			var tracks = dirsAndFiles[1];

			if (lastOnly && i > 0) {
				this.$('.tab[data-name="'+shownDirs[i - 1].path.replace(/"/g, '\\"')+'"] ~').remove();
			}

			var $tab = this.tabs.pushTab(this.template({
				name:   dir.name,
				dirs:   dirs,
				tracks: tracks,
			}), { name: dir.path });

			this.tabs.$el.find('.tab:nth-child('+i+')')
				.find('li.type-dir[data-path="'+dir.path.replace(/"/g, '\\"')+'"]')
				.addClass('active');
			$tab.find('.result-list > li.type-dir').on('click', function() {
				$tab.find('.result-list > li.active').removeClass('active');
				var $li = $(this);
				$li.addClass('active');
				var targetPath = $li.attr('data-path');
				self.showDirectory(targetPath, true);
				self.trigger('change-state', targetPath);
			});
			$tab.find('.result-list > li.type-track').on('click', function() {
				var $li = $(this);
				showInsertionAnimation($li);
				Hotkeys.playerInsert(self.model, tracks[$li.attr('data-index')].track);
			});
			$tab.find('.do-queue-all').on('click', function() {
				var tracks = self.getTracksInDir(dir.path);
				showInsertionAnimation($tab.find('.result-list > li'));
				if (tracks.length < 20 || confirm('You are about to add '+tracks.length+' tracks to the playlist. Is that okay?')) {
					Hotkeys.playerInsert(self.model, tracks);
				}
			});
		}, this);
	},

	getFile: function(path) {
		if (path === '' || path === '/') {
			return this.tree;
		}
		return this.trimSlashes(path).split('/').reduce(function(dir, pathPart) {
			return dir ? dir.files[pathPart] : undefined;
		}, this.tree);
	},

	getTracksInDir: function(path) {
		var self = this;
		if (path === '' || path === '/') {
			return this.model.get('tracks');
		}
		return this.model.get('tracks').filter(function(track) {
			return track.uri.substring(self.commonPath.length).indexOf(path) === 0;
		});
	},

	trimSlashes: function(path) {
		if (path[0] == '/') {
			path = path.substring(1);
		}
		if (path[path.length - 1] == '/') {
			path = path.substring(0, path.length - 1);
		}
		return path;
	},

	join: function() {
		return this.trimSlashes(Array.prototype.join.call(arguments, '/'));
	},

	template: _.template(`
		<div class="browser-files-header">
			<a class="glyphicon glyphicon-arrow-left do-pop-tab"></a>
			<h2 class="do-queue-all"><%- name %>/</h2>
		</div>
		<ul class="result-list">
			<% dirs.forEach(function(file) { %>
				<li class="type-dir" data-path="<%- file.path %>"><%- file.name %></li>
			<% }) %>
			<% tracks.forEach(function(file, i) { %>
				<li class="type-track" data-path-"<%- file.path %>" data-index="<%= i %>" title="<%- formatTrackTitle(file.track) %>">
					<span class="track-artist"><%- file.track.artist %></span>
					<span class="track-title"><%- file.track.title %></span>
					<span class="track-duration"><%- durationToString(file.track.duration) %></span>
					<span class="track-album"><%- file.track.album %></span>
					<span class="glyphicon glyphicon-plus"></span>
				</li>
			<% }) %>
		</ul>
	`),
});
