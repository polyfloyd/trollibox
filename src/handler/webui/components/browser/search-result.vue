<template>
	<li :title="formatTrackTitle(track)">
		<span class="track-artist">
			<template v-for="em in artist">{{ em.head }}<em>{{ em.body }}</em>{{ em.tail }}</template>
		</span>
		<span class="track-title">
			<template v-for="em in title">{{ em.head }}<em>{{ em.body }}</em>{{ em.tail }}</template>
		</span>
		<span class="track-duration">{{ durationToString(track.duration) }}</span>
		<span class="track-album" @click="clickAlbum">
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
		emits: ['click:album'],
		computed: {
			artist() { return this.highlight('artist'); },
			title() { return this.highlight('title'); },
			album() { return this.highlight('album'); },
		},
		methods: {
			highlight(property) {
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
			clickAlbum(event) {
				if (this.track.album) {
					this.$emit('click:album');
					event.stopPropagation();
				}
			},
		},
	}
</script>

<style scoped>
	.track-album:hover {
		text-decoration: underline;
	}
</style>
