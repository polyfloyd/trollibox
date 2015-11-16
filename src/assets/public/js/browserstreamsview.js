'use strict';

var BrowserStreamsView = Backbone.View.extend({
	tagName:   'div',
	className: 'view browser-streams',

	events: {
		'click .do-add-stream':   'doShowAddDialog',
		'click .do-load-default': 'doLoadDefaults',
	},

	initialize: function() {
		this.listenTo(this.model, 'change:streams', this.render);
		this.render();
	},

	render: function() {
		this.$el.html(this.template());

		var $list = this.$('.result-list');
		$list.append(this.model.get('streams').sort(function(a, b) {
			return stringCompareCaseInsensitive(a.album, b.album);
		}).map(function(stream) {
			var self = this;
			var $el = $(this.streamTemplate({
				title: stream.album || stream.id,
			}));
			showTrackArt($el.find('.track-art'), this.model, stream);
			$el.on('click', function() {
				self.model.appendToPlaylist(stream);
			});
			$el.find('.do-remove').on('click', function(event) {
				event.stopPropagation();
				self.model.removeStream(stream);
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
		$dialog.find('input[name="art"]').on('input', function() {
			showTrackArt($dialog.find('.art-preview'), self.model, { art: $(this).val()});
		});
		$dialog.find('form').on('submit', function(event) {
			event.preventDefault();

			var stream = {
				id:    $dialog.find('input[name="url"]').val(),
				album: $dialog.find('input[name="title"]').val(),
				art:   $dialog.find('input[name="art"]').val(),
			};

			function isValidUrl(url) {
				return url.match(/^https?:\/\/.+$/);
			}

			if (!isValidUrl(stream.id)) {
				alert('Stream URL "'+stream.id+'" is invalid');
				return;
			}
			if (stream.art && !isValidUrl(stream.art)) {
				alert('Art URL "'+stream.art+'" is invalid');
				return;
			}

			self.model.addStream(stream);
			$dialog.modal('hide');
		});
	},

	doLoadDefaults: function() {
		if (confirm('Load default stream presets?')) {
			this.model.loadDefaultStreams();
		}
	},

	template: _.template(
		'<div>'+
			'<h2>'+
				'Streams '+
				'<span class="glyphicon glyphicon-plus do-add-stream"></span>'+
				'<span class="do-load-default">load defaults</span>'+
			'</h2>'+
			'<ul class="result-list grid-list"></ul>'+
		'</div>'
	),
	streamTemplate: _.template(
		'<li title="<%- title %>">'+
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
							'<input class="form-control" type="text" name="art" placeholder="Image URL" />'+
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
