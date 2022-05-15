<template>
	<li class="track-item" :class="['queuedby-'+queuedby]" >
		<button class="glyphicon glyphicon-remove do-remove" @click="$emit('remove')"></button>
		<span class="track-artist">{{ artist }}</span><span class="track-title">{{ title }}</span>
	</li>
</template>

<script>
export default {
	props: {
		title: {required: true, type: String},
		artist: String,
		queuedby: {required: true, type: String},
	},
	emits: ['remove'],
}
</script>
