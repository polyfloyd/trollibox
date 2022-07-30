<template>
	<tab-view class="browser-queuer" :tabs="!selectedFilter ? ['list'] : ['list', 'detail']" @pop="tabPop">
		<template #list>
			<h2>AutoQueuer Rules</h2>
			<ul>
				<li :class="{active: activePlayerFilter == ''}" @click="disableAutoQueuer">
					<input type="radio" v-model="activePlayerFilter" value="" />
					None
				</li>
				<li :class="{active: activePlayerFilter == name}" v-for="name in filterList" @click="selectedFilter=name">
					<input type="radio" v-model="activePlayerFilter" :value="name" />
					{{ name }}
				</li>
			</ul>
		</template>
		<template #detail>
			<h2>{{ selectedFilter }}</h2>
			<rule-filter :model-value="filters[selectedFilter]" @update:modelValue="updateSelectedFilter" />
		</template>
	</tab-view>
</template>

<script>
	import PlayerMixin from '../mixins/player.js';
	import RuleFilter from '../filter/rule-filter.vue';
	import TabView from './tab-view.vue';

	export default {
		components: {
			RuleFilter,
			TabView,
		},
		mixins: [PlayerMixin],
		props: {
			urlroot: {required: true, type: String},
			selectedPlayer: {required: true, type: String},
		},
		data() {
			return {
				filters: {},
				selectedFilter: '',
				activePlayerFilter: '',
			};
		},
		async mounted() {
			this._ev = new EventSource(`${this.urlroot}data/filters/events`);
			this._ev.addEventListener(`update`, async event => {
				let {filter, name} = JSON.parse(event.data);
				if (filter.type != 'ruled') {
					console.warning(`Unexpected type for queuer filter: ${filter.type}`);
					return;
				}
				this.filters[name] = filter;
			});
			this._ev.addEventListener(`autoqueuer`, async event => {
				let {player, filter} = JSON.parse(event.data);
				if (player == this.selectedPlayer) {
					this.activePlayerFilter = filter;
				}
			})
		},
		unmounted() {
			this._ev.close();
		},
		computed: {
			filterList() {
				return Object.keys(this.filters).sort();
			},
		},
		watch: {
			async activePlayerFilter(v) {
				await this.setAutoQueuerFilter(v);
			},
		},
		methods: {
			tabPop() {
				this.selectedFilter = '';
			},
			async updateSelectedFilter(filter) {
				let response = await fetch(`${this.urlroot}data/filters/${this.selectedFilter}`, {
					method: 'PUT',
					headers: {
						'Content-Type': 'application/json',
						// Trigger an error response in JSON format.
						'X-Requested-With': 'fetch',
					},
					body: JSON.stringify({filter}),
				});
				if (!response.ok) {
					throw new Error('Could not store filter');
				}
			},
		},
	}
</script>

<style>
</style>
