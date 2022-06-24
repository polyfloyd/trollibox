<template>
	<div class="browser-queuer">
		<h2>AutoQueuer Rules</h2>

		<div>
			<rule-filter :model-value="filter" @update:modelValue="update" />
		</div>
	</div>
</template>

<script>
	import RuleFilter from '../filter/rule-filter.vue';

	export default {
		components: {
			RuleFilter,
		},
		props: {
			urlroot: {required: true, type: String},
		},
		data() {
			return {
				filter: {rules: [], type: 'ruled'},
			};
		},
		mounted() {
			this._ev = new EventSource(`${this.urlroot}data/filters/events`);
			this._ev.addEventListener(`filter:${this.filterName}`, async event => {
				let { filter } = JSON.parse(event.data);
				if (filter.type != 'ruled') {
					console.warning(`Unexpected type for queuer filter: ${filter.type}`);
					return;
				}
				this.filter = filter;
			});
		},
		unmounted() {
			this._ev.close();
		},
		computed: {
			filterName() { return 'queuer'; },
		},
		methods: {
			async update() {
				let response = await fetch(`${this.urlroot}data/filters/${this.filterName}`, {
					method: 'PUT',
					headers: {
						'Content-Type': 'application/json',
						// Trigger an error response in JSON format.
						'X-Requested-With': 'fetch',
					},
					body: JSON.stringify({filter: this.filter}),
				});
				if (!response.ok) {
					throw new Error('Could not store filter');
				}
			},
		},
	}
</script>

<style>
.browser-queuer  {
	flex-flow: column;
}

.browser-queuer > * {
	width: 100%;
}
</style>
