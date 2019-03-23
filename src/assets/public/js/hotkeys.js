'use strict';


var Hotkeys = {
	// Tracks the state of keys being pressed.
	state: {},

	_autorelease: {},

	player: function(player, $scope) {
		$scope.bind('keydown', 'space', function() {
			switch (player.state) {
				case 'paused':
				case 'stopped':
					player.setState('playing');
					break;
				case 'playing':
					player.setState('paused');
					break;
			}
		});

		$scope.bind('keydown', 'esc', function() {
			player.setIndex(1, true);
		});

		var SEEK_STEP = 4;
		$scope.bind('keydown', 'right', function() {
			var cur = player.getCurrentTrack();
			if (!cur) return;
			var pr = player.time + SEEK_STEP;
			player.setTime(pr > cur.duration ? cur.duration : pr < 0 ? 0 : pr);
		});
		$scope.bind('keydown', 'left', function() {
			var cur = player.getCurrentTrack();
			if (!cur) return;
			var pr = player.time - SEEK_STEP;
			player.setTime(pr > cur.duration ? cur.duration : pr < 0 ? 0 : pr);
		});

		var VOL_STEP = 0.05;
		$scope.bind('keydown', 'up', function() {
			var vol = player.volume + VOL_STEP;
			player.setVolume(vol > 100 ? 100 : vol < 0 ? 0 : vol);
		});
		$scope.bind('keydown', 'down', function() {
			var vol = player.volume - VOL_STEP;
			player.setVolume(vol > 100 ? 100 : vol < 0 ? 0 : vol);
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
		if (Hotkeys.state.ctrl && player.index >= 0) {
			player.insertIntoPlaylist(tracks, player.index + 1);
		} else {
			player.appendToPlaylist(tracks);
		}
	},
};

$('body').bind('keydown keyup', function(event) {
	var key = jQuery.hotkeys.specialKeys[event.keyCode];
	Hotkeys.state[key] = event.type == 'keydown';

	// If the user is holding a key while navigating away from the page and
	// then releases said key, this release event is never received. This
	// little hack ensures that the keystate is reset after two seconds.
	clearTimeout(Hotkeys._autorelease[key]);
	Hotkeys._autorelease[key] = setTimeout(function() {
		Hotkeys.state[key] = false;
	}, 2000);
});
