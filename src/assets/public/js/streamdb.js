var StreamDB = NetModel.extend({
	defaults: {
		'streams': [],
	},

	initialize: function(args) {
		this.player = args.player;
		this.attachServerReloader('server-event:update', '/streams', function(data) {
			this.setInternal('streams', data.streams.map(this.player.fillMissingTrackFields, this));
		});
		NetModel.prototype.initialize.call(this, {
			eventSocketPath: '/streams/listen',
		});
	},

	remove: function(stream) {
		this.callServer('/streams?uri='+encodeURIComponent(stream.uri), 'DELETE');
	},

	add: function(stream) {
		this.callServer('/streams', 'POST', { stream: stream });
	},
});
