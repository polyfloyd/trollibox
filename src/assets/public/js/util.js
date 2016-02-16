'use strict';

// https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Array/findIndex
Array.prototype.findIndex = Array.prototype.findIndex || function(predicate) {
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

$.fn.lazyLoad = function(list, render, thisArg) {
	var $list = this;
	$list.off('scroll.lazy mousewheel.lazy DOMMouseScroll.lazy');
	$list.attr('lazy-index', '0');
	$list.empty();
	var handler = function() {
		var elemSize = $list.children().last().outerHeight();
		while ($list.scrollTop() >= $list[0].scrollHeight - $list.innerHeight() - elemSize) {
			var listIndex = parseInt($list.attr('lazy-index'), 10);
			if (listIndex >= list.length) {
				$list.off('scroll.lazy mousewheel.lazy DOMMouseScroll.lazy');
				return;
			}
			$list.append(render.call(thisArg, list[listIndex]));
			$list.attr('lazy-index', listIndex + 1);
			elemSize = $list.children().last().outerHeight();
		}
	};
	this.on('scroll.lazy mousewheel.lazy DOMMouseScroll.lazy', handler);
	handler();
};

function durationToString(seconds) {
	var parts = [];
	for (var i = 1; i <= 3; i++) {
		var l = seconds % Math.pow(60, i - 1);
		parts.push((seconds % Math.pow(60, i) - l) / Math.pow(60, i - 1));
	}

	// Don't show hours if there are none.
	if (parts[2] === 0) {
		parts.pop();
	}

	return parts.reverse().map(function(p) {
		return (p<10 ? '0' : '')+p;
	}).join(':');
}

function showTrackArt($elem, player, track, cb) {
	$elem.css('background-image', ''); // Reset to default.
	if (!track || !track.uri) {
		if (cb) cb(false);
		return;
	}

	var url = URLROOT+'data/player/'+player.name+'/art?track='+encodeURIComponent(track.uri).replace(/'/g, '%27');
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
