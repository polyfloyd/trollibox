<template>
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
			<search-result v-for="(result, i) in results" :key="i"
				v-bind="result"
				@click.native="appendToPlaylist(result.track, $event)" />
		</ul>
	</div>
</template>

<script>
	import ApiMixin from '../mixins/api.js';
	import SearchResult from './search-result.vue';
  import PlaylistMixin from '../mixins/playlist.js';

	export default {
		components: {
			SearchResult,
		},
		mixins: [ApiMixin, PlaylistMixin],
		data: function() {
			return {
				untagged: ['artist', 'title', 'album'],
				query: '',
				results: [],
				ctx: null,
			};
		},
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
	}
</script>

<style>
.browser-search {
	flex-direction: column;
}

.browser-search > .search-input {
	flex-shrink: 0;
	margin-bottom: 15px;
}

.browser-search > .result-list {
	height: 100%;
}

.browser-search.search-busy > .result-list:empty:before {
	content: "Searching...";
}
</style>
