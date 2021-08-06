Vue.component('browser-search-result', {
	mixins: [TrackMixin],
	props: {
		track: {required: true, type: Object},
		matches: {required: true, type: Object},
	},
	template: `
		<li :title="formatTrackTitle(track)">
			<span class="track-artist"><span v-html="highlight('artist')" /></span>
			<span class="track-title"><span v-html="highlight('title')" /></span>
			<span class="track-duration">{{ durationToString(track.duration) }}</span>
			<span class="track-album"><span v-html="highlight('album')" /></span>
			<span class="glyphicon glyphicon-plus"></span>
		</li>
	`,
	methods: {
		highlight: function(property) {
			let propMatches = [].concat(this.matches[property] || []);
			let value = propMatches
				.sort((a, b) => a.start - b.end)
				.reduce((state, match) => {
					// Ensure that matches don't overlap each other.
					let newStart = [state.prevEnd, match.start, match.end]
						.sort((a, b) => a - b)[1];
					state.prevEnd = match.end;
					if (newStart == match.end) return state;
					state.noOverlap.push({start: newStart, end: match.end});
					return state;
				}, { noOverlap: [], prevEnd: 0 })
				.noOverlap
				.reduceRight((value, match) => {
					return value.substring(0, match.start)+'<em>'+value.substring(match.start, match.end)+'</em>'+value.substring(match.end);
				}, this.track[property]);
			return _.escape(value).replace(/&lt;(\/)?em&gt;/g, '<$1em>');
		},
	},
});


Vue.component('browser-search', {
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
					@click.native="appendToPlaylist(result.track, $event.target)" />
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

			} catch (AbortError) {
				return;
			}
		},
	},
});
