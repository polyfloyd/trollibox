var StreamDB = NetModel.extend({
	defaults: {
		'streams': [],
	},

	initialize: function(args) {
		this.player = args.player;
		this.attachServerReloader('server-event:tracks', '/streams', function(data) {
			this.setInternal('streams', data.streams.map(function(stream) {
				stream.uri = stream.url;
				return this.player.fillMissingTrackFields(stream);
			}, this));
		});
		NetModel.prototype.initialize.call(this, {
			eventSocketPath: '/streams/listen',
		});
	},

	remove: function(stream) {
		this.callServer('/streams?filename='+encodeURIComponent(stream.filename), 'DELETE');
	},

	add: function(stream) {
		this.callServer('/streams', 'POST', { stream: stream });
	},
});
