'use strict';

(function() {

var WorkerPool = function(script) {
	this.script = script;
	this.pool = [];
};

WorkerPool.prototype.get = function() {
	if (!this.pool.length) {
		return new Worker(this.script);
	}
	return this.pool.pop();
};

WorkerPool.prototype.put = function(worker) {
	worker.onmessage = null;
	this.pool.push(worker);
};


function _search(keywords, tracks) {
	return tracks.reduce(function(list, track) {
		var numMatches = 0;
		var allMatch = keywords.every(function(keyword) {
			return ['artist', 'title', 'album'].filter(function(attr) {
				var val = track[attr];
				if (!val) {
					return false;
				}
				var match = val.toLowerCase().indexOf(keyword) !== -1;
				numMatches += match ? 1 : 0; //  Meh, no functional for you!
				return match;
			}).length > 0;
		});

		if (allMatch) {
			// A concat() would be more correcter from a functional
			// perspective, but also A LOT slower! :(
			list.push({
				matches: numMatches,
				track:   track,
			});
		}
		return list
	}, []);
}

function _searchScript() {
	onmessage = function(event) {
		postMessage({
			tracks: _search(event.data.keywords, event.data.tracks),
		});
	};
}


var blob = new Blob([_search.toString()+'('+_searchScript.toString()+')()']);
var _searchPool = new WorkerPool(window.URL.createObjectURL(blob));

window.searchTracks = function(query, tracks, cb) {
	var keywords = query.toLowerCase().split(/\s+/g).filter(function(keyword) {
		return !!keyword;
	});
	if (!keywords.length) {
		cb([]);
		return;
	}

	var chunksSize = 5000;
	var chunks = [];
	for (var i = 0; i < tracks.length; i += chunksSize) {
		chunks.push(tracks.slice(i, i + chunksSize));
	}

	if (chunks.length <= 1) {
		cb(_search(keywords, chunks[0]));
		return
	}

	var done = 0;
	var result = [];
	chunks.forEach(function(chunk) {
		var worker = _searchPool.get();
		worker.postMessage({
			tracks:   chunk,
			keywords: keywords,
		});
		worker.onmessage = function(event) {
			_searchPool.put(worker);

			result = result.concat(event.data.tracks);
			if (++done == chunks.length) {
				cb(result);
			}
		};
	});
}

})();
