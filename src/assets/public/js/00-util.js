'use strict';

jQuery.fn.toggleAttr = function(attr, value, toggle) {
	if (typeof toggle === 'undefined') {
		toggle = value;
		value = undefined;
	}
	if (typeof value === 'undefined') {
		value = attr;
	}

	return this.each(() => {
		var $this = $(this);
		if (toggle) {
			$this.attr(attr, value);
		} else {
			$this.removeAttr(attr);
		}
	});
};

$.fn.lazyLoad = function(list, render, thisArg, overrideElemSize) {
	var $list = this;
	$list.off('scroll.lazy mousewheel.lazy DOMMouseScroll.lazy');
	$list.attr('lazy-index', '0');
	$list.empty();
	var handler = () => {
		var elemSize = overrideElemSize || $list.children().last().outerHeight();
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

	return parts.reverse().map((p) => {
		return (p<10 ? '0' : '')+p;
	}).join(':');
}

var _formatTrackTitleTemplate = _.template(`<% if (albumtrack) {%><%= albumtrack %>.  <% } %><% if (artist) {%><%= artist %> - <% } %><%= title %><% if (duration) {%> (<%= durationToString(duration) %>)<% } %>`);

function formatTrackTitle(track) {
	return  _formatTrackTitleTemplate({
		artist: track.artist || '',
		title: track.title || '',
		albumtrack: track.albumtrack || '',
		duration: track.duration || 0,
	});
}

function showTrackArt($elem, player, track) {
	$elem.css('background-image', ''); // Reset to default.
	if (!track || !track.uri || !track.hasart) {
		return Promise.resolve();
	}

	return fetch(URLROOT+'data/player/'+player.name+'/tracks/art?track='+encodeURIComponent(track.uri))
		.then((res) => {
			if (res.status >= 400) {
				throw new Error('could not fetch track art for '+track.uri);
			}
			return res.blob();
		})
		.then((blob) => {
			let url = URL.createObjectURL(blob);
			$elem.css('background-image', 'url(\''+url+'\')');
		});
}

/**
 * Shows an animation to indicate that a track was added to the playlist.
 */
function showInsertionAnimation($elems) {
	$elems.each((i, el) => {
		setTimeout(() => {
			var $elem = $(el);
			var $anim = $('<div class="insertion-animation glyphicon glyphicon-plus"></div>');
			$anim.css($elem.offset());

			$('body').prepend($anim);
			setTimeout(() => {
				$anim.remove();
			}, 1500);
		}, i * 40);
	});
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
