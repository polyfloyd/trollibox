<template>
	<li :title="formatTrackTitle(track)">
		<span class="track-artist">
			<template v-for="em in artist">{{ em.head }}<em>{{ em.body }}</em>{{ em.tail }}</template>
		</span>
		<span class="track-title">
			<template v-for="em in title">{{ em.head }}<em>{{ em.body }}</em>{{ em.tail }}</template>
		</span>
		<span class="track-duration">{{ durationToString(track.duration) }}</span>
		<span class="track-album" @click.stop="$emit('click:album')">
			<template v-for="em in album">{{ em.head }}<em>{{ em.body }}</em>{{ em.tail }}</template>
		</span>
		<span class="glyphicon glyphicon-plus"></span>
	</li>
</template>

<script>
	import TrackMixin from '../mixins/track.js';

	export default {
		mixins: [TrackMixin],
		props: {
			track: {required: true, type: Object},
			matches: {required: true, type: Object},
		},
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
	}
</script>

<style scoped>
	.track-album:hover {
		text-decoration: underline;
	}
</style>
