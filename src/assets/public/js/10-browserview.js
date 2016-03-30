'use strict';

var BrowserView = Backbone.View.extend({
	setState: function(state) {
		this.trigger('change-state', state || '');
	},

	getState: function() { return ''; },
});
