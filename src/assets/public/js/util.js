'use strict';

// https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Array/findIndex
Array.prototype.findIndex =  Array.prototype.findIndex || function(predicate) {
	if (this == null) {
		throw new TypeError('Array.prototype.findIndex called on null or undefined');
	}
	if (typeof predicate !== 'function') {
		throw new TypeError('predicate must be a function');
	}
	var list = Object(this);
	var length = list.length >>> 0;
	var thisArg = arguments[1];
	var value;

	for (var i = 0; i < length; i++) {
		value = list[i];
		if (predicate.call(thisArg, value, i, list)) {
			return i;
		}
	}
	return -1;
};

function durationToString(seconds) {
	var s = '';
	var hasHours = seconds > 3600;
	if (hasHours) {
		s += Math.round(seconds / 3600)+':';
		seconds %= 3600;
	}
	var min = Math.round(seconds / 60 - 0.5);
	if (min < 10 && hasHours) {
		s += '0';
	}
	s += min+':';
	var sec = seconds % 60;
	if (sec < 10) {
		s += '0';
	}
	return s + sec;
}

function showTrackArt($elem, track, cb) {
	$elem.css('background-image', ''); // Reset to default.
	if (!track || !track.id) {
		return;
	}

	var url = URLROOT+'data/track/art/'+encodeURIComponent(track.id);
	$.ajax({
		method:   'HEAD',
		url:      url,
		complete: function(xhr, state) {
			if (state !== 'error') {
				$elem.css('background-image', 'url(\''+url.replace('\'', '\\\'')+'\')');
			}
			if (cb) cb(state !== 'error');
		},
	});
}
