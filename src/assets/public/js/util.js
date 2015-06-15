'use strict';

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
