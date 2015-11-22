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

jQuery.fn.toggleAttr = function(attr, value, toggle) {
	if (typeof toggle === 'undefined') {
		toggle = value;
		value = undefined;
	}
	if (typeof value === 'undefined') {
		value = attr;
	}

	return this.each(function() {
		var $this = $(this);
		if (toggle) {
			$this.attr(attr, value);
		} else {
			$this.removeAttr(attr);
		}
	});
};

$.fn.lazyLoad = function(callback, thisArg) {
	thisArg = thisArg || this;
	var $el = this;
	this.on('scroll mousewheel DOMMouseScroll', function(event) {
		if ($el.scrollTop() == $el[0].scrollHeight - $el.innerHeight()) {
			callback.call(thisArg, event);
		}
	});
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

function showTrackArt($elem, player, track, cb) {
	$elem.css('background-image', ''); // Reset to default.
	if (!track || !track.id) {
		if (cb) cb(false);
		return;
	}

	var url = URLROOT+'data/player/'+player.name+'/art?track='+encodeURIComponent(track.id).replace(/'/g, '%27');
	if (track.hasart) {
		$elem.css('background-image', 'url(\''+url+'\')');
	}
	if (cb) cb(track.hasart)
}

/**
 * To be used as argument to Array#sort(). Compares strings without case
 * sensitivity.
 */
function stringCompareCaseInsensitive(a, b) {
	a = a.toLowerCase();
	b = b.toLowerCase();
	return a > b ? 1
		: a < b ? -1
		: 0;
}
