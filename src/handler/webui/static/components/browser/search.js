app.component('browser-search-result', {
	mixins: [TrackMixin],
	props: {
		track: {required: true, type: Object},
		matches: {required: true, type: Object},
	},
	template: `
		<li :title="formatTrackTitle(track)">
			<span class="track-artist">
				<template v-for="em in artist">{{ em.head }}<em>{{ em.body }}</em>{{ em.tail }}</template>
			</span>
			<span class="track-title">
				<template v-for="em in title">{{ em.head }}<em>{{ em.body }}</em>{{ em.tail }}</template>
			</span>
			<span class="track-duration">{{ durationToString(track.duration) }}</span>
			<span class="track-album">
				<template v-for="em in album">{{ em.head }}<em>{{ em.body }}</em>{{ em.tail }}</template>
			</span>
			<span class="glyphicon glyphicon-plus"></span>
		</li>
	`,
	computed: {
		artist: function() { return this.highlight('artist'); },
		title: function() { return this.highlight('title'); },
		album: function() { return this.highlight('album'); },
	},
	methods: {
		highlight: function(property) {
			let {sections, head} = [].concat(this.matches[property] || [])
				.sort((a, b) => a.start - b.end)
				// Ensure that matches don't overlap each other.
				.reduce((state, match) => {
					let newStart = [state.prevEnd, match.start, match.end]
						.sort((a, b) => a - b)[1];
					state.prevEnd = match.end;
					if (newStart == match.end) return state;
					state.noOverlap.push({start: newStart, end: match.end});
					return state;
				}, { noOverlap: [], prevEnd: 0 })
				.noOverlap
				// Map matches into string sections.
				// The value string is progressively consumed by each
				// iteration, yielding a non-highlighted tail, the highlighted body
				// and the remaining head passed to the next itertaion.
				.reduceRight(({head, sections}, match) => {
					let body = head.substring(match.start, match.end);
					let tail = head.substring(match.end);
					let nextHead = head.substring(0, match.start);
					sections.push({body, tail});
					return {head: nextHead, sections};
				}, {head: this.track[property], sections: []});
			return [{tail: head}].concat(sections.reverse());
		},
	},
});


app.component('browser-search', {
	mixins: [ApiMixin, PlaylistMixin],
	data: function() {
		return {
			untagged: ['artist', 'title', 'album'],
			query: '',
			results: [],
			ctx: null,
		};
	},
	template: `
		<div class="browser-search" :class="{'search-busy': ctx}">
			<div class="search-input">
				<div class="input-group">
					<span class="input-group-addon">
						<span class="glyphicon glyphicon-search"></span>
					</span>
					<input v-model.trim="query"
						class="form-control input-lg"
						type="text"
						placeholder="Search the Library" />
				</div>
			</div>
			<ul class="result-list search-results">
				<browser-search-result v-for="(result, i) in results" :key="i"
					v-bind="result"
					@click.native="appendToPlaylist(result.track, $event)" />
			</ul>
		</div>
	`,
	mounted: function() {
		this.$el.querySelector('input').focus();
	},
	watch: {
		query: async function() {
			if (this.ctx) {
				this.ctx.abort();
				this.ctx = null;
			}
			if (this.query == '') {
				this.results = [];
				return;
			}

			const encUt = encodeURIComponent(this.untagged.join(','));
			const encQ = encodeURIComponent(this.query);
			let url = `${this.urlroot}data/player/${this.selectedPlayer}/tracks/search?query=${encQ}&untagged=${encUt}`;

			try {
				this.ctx = new AbortController();
				let response = await fetch(url, {signal: this.ctx.signal});
				if (response.status != 200) {
					throw new Error(`could not perform search: http status ${response.status}`);
				}
				let { tracks } = await response.json();
				this.results = tracks.slice(0, 200); // TODO: Remove subslice?
				this.ctx = null;

			} catch (AbortError) {
				// Do not unset ctx, it has been overwritten by a new search
				// query.
				return;
			}
		},
	},
});
