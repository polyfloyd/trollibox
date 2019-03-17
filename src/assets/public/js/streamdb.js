var StreamDB = NetModel.extend({
	defaults: {
		'streams': [],
	},

	initialize: function(args) {
		NetModel.prototype.initialize.call(this, {
			eventSourcePath: '/streams/events',
		});
		this.player = args.player;
		this.attachServerReloader('server-event:tracks', '/streams', (data) => {
			this.setInternal('streams', data.streams.map((stream) => {
				stream.uri = stream.url;
				return this.player.fillMissingTrackFields(stream);
			}, this));
		});
	},

	remove: function(stream) {
		this.callServer('/streams?filename='+encodeURIComponent(stream.filename), 'DELETE');
	},

	add: function(stream) {
		this.callServer('/streams', 'POST', { stream: stream });
	},
});
