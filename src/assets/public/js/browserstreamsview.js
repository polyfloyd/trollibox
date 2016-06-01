'use strict';

var BrowserStreamsView = BrowserView.extend({
	tagName:   'div',
	className: 'view browser-streams',

	events: {
		'click .do-add-stream': 'doShowAddDialog',
	},

	initialize: function(args) {
		this.player = args.player;
		this.listenTo(this.model, 'change:streams', this.render);
		this.render();
	},

	render: function() {
		this.$el.html(this.template());

		var $list = this.$('.result-list');
		$list.append(this.model.get('streams').sort(function(a, b) {
			return stringCompareCaseInsensitive(a.title, b.title);
		}).map(function(stream) {
			var self = this;
			var $el = $(this.streamTemplate({
				title: stream.title || stream.url,
			}));
			showTrackArt($el.find('.track-art'), this.player, stream);
			$el.on('click', function() {
				Hotkeys.playerInsert(self.player, [stream]);
			});
			$el.find('.do-remove').on('click', function(event) {
				event.stopPropagation();
				self.model.remove(stream);
			});
			return $el;
		}, this));
	},

	doShowAddDialog: function() {
		var self = this;

		var $dialog = $(this.addStreamDialog()).modal();
		$dialog.on('hidden.bs.modal', function() {
			$dialog.remove();
		});
		$dialog.find('input[name="arturi"]').on('input', function() {
			showTrackArt($dialog.find('.art-preview'), self.player, { art: $(this).val() });
		});
		$dialog.find('form').on('submit', function(event) {
			event.preventDefault();

			var stream = {
				url:    $dialog.find('input[name="url"]').val(),
				title:  $dialog.find('input[name="title"]').val(),
				arturi: $dialog.find('input[name="arturi"]').val(),
			};

			function isValidUrl(url) {
				return url.match(/^https?:\/\/.+$/);
			}

			if (!isValidUrl(stream.url)) {
				alert('Stream URL "'+stream.url+'" is invalid');
				return;
			}
			if (stream.arturi && !isValidUrl(stream.arturi)) {
				alert('Art URL "'+stream.arturi+'" is invalid');
				return;
			}

			self.model.add(stream);
			$dialog.modal('hide');
		});
	},

	template: _.template(
		'<h2>'+
			'Streams '+
			'<span class="glyphicon glyphicon-plus do-add-stream"></span>'+
		'</h2>'+
		'<ul class="result-list grid-list"></ul>'
	),
	streamTemplate: _.template(
		'<li title="<%- title %>">'+
			'<img class="ratio" src="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAABmJLR0QA/wD/AP+gvaeTAAAACXBIWXMAAAsTAAALEwEAmpwYAAAADUlEQVQI12NgYGBgAAAABQABXvMqOgAAAABJRU5ErkJggg==" />'+
			'<div class="track-art">'+
				'<span class="stream-title"><%- title %></span>'+
				'<button class="glyphicon glyphicon-remove do-remove"></button>'+
			'</div>'+
		'</li>'
	),
	addStreamDialog: _.template(
		'<div class="modal fade">'+
			'<div class="modal-dialog">'+
				'<form class="modal-content dialog-add-stream">'+
					'<div class="modal-header">'+
						'<button type="button" class="close" data-dismiss="modal" aria-label="Close"><span aria-hidden="true">&times;</span></button>'+
						'<h4 class="modal-title">Add Stream</h4>'+
					'</div>'+
					'<div class="modal-body">'+
						'<div class="input-group">'+
							'<input class="form-control" type="text" name="url" placeholder="URL" required />'+
							'<input class="form-control" type="text" name="title" placeholder="Title" required />'+
							'<input class="form-control" type="text" name="arturi" placeholder="Image URL" />'+
						'</div>'+
						'<div class="art-preview track-art"></div>'+
					'</div>'+
					'<div class="modal-footer">'+
						'<button type="button" class="btn btn-default" data-dismiss="modal">Cancel</button>'+
						'<input type="submit" class="btn btn-default do-add" value="Add" />'+
					'</div>'+
				'</form>'+
			'</div>'+
		'</div>'
	),
});
