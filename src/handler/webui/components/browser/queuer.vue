<template>
	<tab-view class="browser-queuer" :tabs="!selectedFilter ? ['list'] : ['list', 'detail']" @pop="tabPop">
		<template #list>
			<h2>Auto Queuers</h2>
			<ul class="result-list">
				<li :class="{active: selectedFilter == ''}" @click="selectedFilter=''">
					<input type="radio" v-model="activePlayerFilter" value="" title="Disable autoqueuer" />
					None
				</li>
				<li :class="{active: selectedFilter == name}" v-for="name in filterList" :key="name" @click="selectedFilter=name">
					<input type="radio" v-model="activePlayerFilter" :value="name" title="Attach to player" />
					{{ name }}
					<span class="rule-count">({{ filters[name].rules.length}} rules)</span>
				</li>
			</ul>

			<div class="input-group add-filter">
				<input type="text" v-model.trim="newFilterNameRaw" class="form-control" placeholder="New filter name" />
				<span class="input-group-btn">
					<button class="btn btn-default" type="button" @click="addFilter"
						:disabled="!newFilterName">
						<span class="glyphicon glyphicon-plus"></span>
						Add
					</button>
				</span>
			</div>

		</template>
		<template #detail>
			<h2>{{ selectedFilter }}</h2>
			<rule-filter :model-value="filters[selectedFilter]" @update:modelValue="updateSelectedFilter" />
			<div class="remove-filter">
				<button class="btn btn-default" @click="deleteSelectedFilter">
					<span class="glyphicon glyphicon-trash"></span>
					Delete Filter
				</button>
			</div>
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
				newFilterNameRaw: '',
			};
		},
		async mounted() {
			this._ev = new EventSource(`${this.urlroot}data/filters/events`);
			this._ev.addEventListener(`update`, async event => {
				let {filter, name} = JSON.parse(event.data);
				if (!filter) {
					if (this.selectedFilter == name) {
						this.selectedFilter = '';
					}
					delete this.filters[name];
					return
				}
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
			});
		},
		unmounted() {
			this._ev.close();
		},
		computed: {
			filterList() {
				return Object.keys(this.filters).sort();
			},
			newFilterName() {
				if (!this.newFilterNameRaw.match(/^\w+$/)) return;
				return this.newFilterNameRaw;
			},
		},
		watch: {
			async activePlayerFilter(v) {
				await this.setAutoQueuerFilter(v);
				if (!this.selectedFilter) {
					this.selectedFilter = v;
				}
			},
		},
		methods: {
			tabPop() {
				this.selectedFilter = '';
			},
			async addFilter() {
				if (!this.newFilterName) return;
				await this.setFilter(this.newFilterName, {type: 'ruled', rules:[]});
				this.newFilterNameRaw = '';
			},
			async deleteSelectedFilter() {
				let response = await fetch(`${this.urlroot}data/filters/${this.selectedFilter}`, {
					method: 'DELETE',
				});
				if (!response.ok) {
					throw new Error('Could not delete filter');
				}
			},
			async updateSelectedFilter(filter) {
				await this.setFilter(this.selectedFilter, filter);
			},
			async setFilter(filterName, filter) {
				let response = await fetch(`${this.urlroot}data/filters/${filterName}`, {
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

<style lang="scss">
	.browser-queuer {
		h2 {
			margin-top: 0;
		}

		li input[type="radio"] {
			margin: 0 0.2em;
		}

		li .rule-count {
			margin-left: 0.2em;
			color: var(--color-text-inactive);
		}

		.input-group.add-filter {
			margin-top: 1em;
		}

		.remove-filter {
			margin-top: 1em;
		}
	}
</style>
