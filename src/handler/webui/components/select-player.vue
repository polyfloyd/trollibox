<template>
	<select class="select-player" v-model="selectedPlayer">
		<option v-for="player in players" :value="player.name">{{ player.name }}</option>
	</select>
</template>

<script>
	export default {
		props: {
			urlroot: {required: true, type: String},
			players: {required: true, type: Array},
			initialSelectedPlayer: {required: true, type: String},
		},
		data: function() {
			return {selectedPlayer: this.initialSelectedPlayer};
		},
		watch: {
			selectedPlayer: function(value) {
				window.location = `${this.urlroot}player/${value}`;
			},
		},
	}
</script>
