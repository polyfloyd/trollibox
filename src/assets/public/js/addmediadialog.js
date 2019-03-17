'use strict';

var AddMediaDialog = Backbone.View.extend({
	tagName:   'div',
	className: 'modal fade',

	initialize: function() {
		this.render();
	},

	render: function() {
		this.$el.html(this.template()).modal();

		this.$('.do-add-netmedia').on('click', (event) => {
			var url = this.$('.mediadialog-value-netmedia').val();
			this.model.playFromNetwork(url).then(() => {
				this.$el.modal('hide');
			}).catch((err) => {
				this.$('.error-message').text(err.message);
			});
		});
		this.$('.do-add-files').on('click', (event) => {
			var files = this.$('.mediadialog-value-files')[0].files;
			this.model.playRawTracks(files).then(() => {
				this.$el.modal('hide');
			}).catch((err) => {
				this.$('.error-message').text(err.message);
			});
		});
	},

	template: _.template(`
		<div class="modal-dialog">
			<div class="modal-content">
				<div class="modal-header">
					<button type="button" class="close" data-dismiss="modal" aria-label="Close"><span aria-hidden="true">&times;</span></button>
					<h4 class="modal-title">Add Media</h4>
				</div>

				<div class="modal-body">
					<p class="error-message">
					<div class="form-group">
						<p>
							Add from URL (
							<img src="${window.URLROOT}img/icon-youtube-16.png" />,
							<img src="${window.URLROOT}img/icon-soundcloud-16.png" />
							)
						</p>
						<div class="input-group">
							<span class="input-group-addon">
								<span class="glyphicon glyphicon-search"></span>
							</span>
							<input class="form-control mediadialog-value-netmedia" type="text" name="url" placeholder="URL" />
							<span class="input-group-btn">
								<button type="button" class="btn btn-default do-add-netmedia">Add</button>
							</span>
						</div>
					</div>

					<div class="form-group">
						<p>Upload File</p>
						<div class="input-group">
							<span class="input-group-addon">
								<span class="glyphicon glyphicon-file"></span>
							</span>
							<input class="form-control mediadialog-value-files" type="file" name="url" multiple accept="audio/*" />
							<span class="input-group-btn">
								<button class="btn btn-default do-add-files">Add</button>
							</span>
						</div>
					</div>
				</div>
				<div class="modal-footer">
					<button type="button" class="btn btn-default" data-dismiss="modal">Cancel</button>
				</div>
			</div>
		</div>'
	`),
});
