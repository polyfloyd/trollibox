'use strict';


var Hotkeys = {
	state: {},

	player: function(player, $scope) {
		$scope.bind('keydown', 'space', function() {
			switch (player.get('state')) {
				case 'paused':
				case 'stopped':
					player.set('state', 'playing');
					break;
				case 'playing':
					player.set('state', 'paused');
					break;
			}
		});

		$scope.bind('keydown', 'esc', function() {
			player.next();
		});

		var SEEK_STEP = 4;
		$scope.bind('keydown', 'right', function() {
			var cur = player.get('current');
			if (!cur) return;
			var pr = player.get('time') + SEEK_STEP;
			player.set('time', pr > cur.duration ? cur.duration : pr < 0 ? 0 : pr);
		});
		$scope.bind('keydown', 'left', function() {
			var cur = player.get('current');
			if (!cur) return;
			var pr = player.get('time') - SEEK_STEP;
			player.set('time', pr > cur.duration ? cur.duration : pr < 0 ? 0 : pr);
		});

		var VOL_STEP = 0.05;
		$scope.bind('keydown', 'up', function() {
			var vol = player.get('volume') + VOL_STEP;
			player.set('volume',  vol > 100 ? 100 : vol < 0 ? 0 : vol);
		});
		$scope.bind('keydown', 'down', function() {
			var vol = player.get('volume') - VOL_STEP;
			player.set('volume',  vol > 100 ? 100 : vol < 0 ? 0 : vol);
		});
	},

	browserSearch: function(view, $scope) {
		$scope.bind('keydown', '/', function() {
			setTimeout(function() {
				view.focusInput();
			}, 1);
		});
	},

	playerInsert: function(player, tracks) {
		var cur = player.get('current');
		if (Hotkeys.state.ctrl && cur > 0) {
			player.insertIntoPlaylist(tracks, cur + 1);
		} else {
			player.appendToPlaylist(tracks);
		}
	},
};

$('body').bind('keydown keyup', function(event) {
	var key = jQuery.hotkeys.specialKeys[event.keyCode];
	Hotkeys.state[key] = event.type == 'keydown';
});
