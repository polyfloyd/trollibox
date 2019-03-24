'use strict';

var BrowserStreamsView = BrowserView.extend({
	tagName:   'div',
	className: 'view browser-streams',

	events: {
		'click .do-add-stream': 'doShowAddDialog',
	},

	initialize: function(args) {
		this.player = args.player;
		this.model.addEventListener('change:streams', () => this.render());
		this.render();
	},

	render: function() {
		this.$el.html(this.template());

		var $list = this.$('.result-list');
		$list.append(this.model.streams.sort(function(a, b) {
			return stringCompareCaseInsensitive(a.title, b.title);
		}).map(function(stream) {
			var self = this;
			var $el = $(this.streamTemplate({
				title: stream.title || stream.url,
			}));
			showTrackArt($el.find('.track-art'), this.player, stream);
			$el.on('click', function() {
				showInsertionAnimation($el);
				Hotkeys.playerInsert(self.player, [stream]);
			});
			$el.find('.do-edit').on('click', function(event) {
				event.stopPropagation();
				self.showEditDialog(stream);
			});
			$el.find('.do-remove').on('click', function(event) {
				event.stopPropagation();
				self.model.remove(stream);
			});
			return $el;
		}, this));
	},

	showEditDialog: function(stream) {
		var self = this;

		var $dialog = $(this.editStreamDialog(stream || { url: '', title: '', hasart: false })).modal();
		showTrackArt($dialog.find('.art-preview'), self.player, stream);
		$dialog.on('hidden.bs.modal', function() {
			$dialog.remove();
		});
		$dialog.find('input[name="arturi"]').on('input', function() {
			var newArtURL = $(this).val();
			if (newArtURL) {
				$dialog.find('.art-preview').css('background-image', 'url(\''+newArtURL+'\')');
			} else {
				showTrackArt($dialog.find('.art-preview'), self.player, stream);
			}
		});
		$dialog.find('form').on('submit', function(event) {
			event.preventDefault();

			var newStream = {
				filename: stream ? stream.filename : '',
				url:      $dialog.find('input[name="url"]').val(),
				title:    $dialog.find('input[name="title"]').val(),
				arturi:   $dialog.find('input[name="arturi"]').val(),
			};

			function isValidUrl(url) {
				return url.match(/^https?:\/\/.+$/);
			}
			if (!isValidUrl(newStream.url)) {
				alert('Stream URL "'+newStream.url+'" is invalid');
				return;
			}
			if (newStream.arturi && !isValidUrl(newStream.arturi)) {
				alert('Art URL "'+newStream.arturi+'" is invalid');
				return;
			}

			self.model.add(newStream);
			$dialog.modal('hide');
		});
	},

	doShowAddDialog: function() {
		this.showEditDialog(null);
	},

	template: _.template(`
		<h2>
			Network Streams
			<span class="glyphicon glyphicon-plus do-add-stream"></span>
		</h2>
		<ul class="result-list grid-list"></ul>
	`),
	streamTemplate: _.template(`
		<li title="<%- title %>">
			<img class="ratio" src="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAABmJLR0QA/wD/AP+gvaeTAAAACXBIWXMAAAsTAAALEwEAmpwYAAAADUlEQVQI12NgYGBgAAAABQABXvMqOgAAAABJRU5ErkJggg==" />
			<div class="track-art">
				<span class="stream-title">
					<%- title %>
				</span>
				<span class="glyphicon glyphicon-plus do-add"></span>
				<button class="glyphicon glyphicon-remove do-remove"></button>
				<button class="glyphicon glyphicon-edit do-edit"></button>
			</div>
		</li>
	`),
	editStreamDialog: _.template(`
		<div class="modal fade">
			<div class="modal-dialog">
				<form class="modal-content dialog-add-stream">
					<div class="modal-header">
						<button type="button" class="close" data-dismiss="modal" aria-label="Close"><span aria-hidden="true">&times;</span></button>
						<h4 class="modal-title">Add Stream</h4>
					</div>
					<div class="modal-body">
						<div class="input-group">
							<input class="form-control" type="text" name="url" value="<%- url %>" placeholder="URL" required />
							<input class="form-control" type="text" name="title" value="<%- title %>" placeholder="Title" required />
							<input class="form-control" type="text" name="arturi" placeholder="<%- hasart ? "Keep current image URL" : "Image URL" %>" />
						</div>
						<div class="art-preview track-art"></div>
					</div>
					<div class="modal-footer">
						<button type="button" class="btn btn-default" data-dismiss="modal">Cancel</button>
						<input type="submit" class="btn btn-default do-add" value="Add" />
					</div>
				</form>
			</div>
		</div>
	`),
});
