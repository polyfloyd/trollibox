<template>
	<div class="tab-view">
		<div v-for="(tab, index) in tabs" class="tab" :class="[`tab-${tab}`]">
			<a v-if="index > 0" class="glyphicon glyphicon-arrow-left pop-tab" @click="pop(tab)"></a>
			<slot :name="tab"></slot>
		</div>
	</div>
</template>

<script>
	export default {
		props: {
			tabs: {type: Array, required: true,
				validate: v => v.length > 0 && v.every(n => n.matches(/^\w+$/))},
		},
		emits: ['pop'],
		methods: {
			pop(tab) {
				this.$emit('pop', tab);
			},
		},
	}
</script>

<style scoped>
	.tab-view {
		width: 100%;
		padding: 0;
		display: flex;
		background-color: transparent;
		overflow: hidden; /* Hide the overflowing :after */
	}

	.tab {
		width: 50%;
		height: 100%;
		float: left;
		display: flex;
		flex-direction: column;
		overflow-y: auto;
	}

	.pop-tab {
		font-size: 2.2rem;
		padding: 0 0.3em 0 0;
		margin-bottom: 0.4em;
		font-size: 20px;
		float: right;
		color: var(--color-accent);
		cursor: pointer;
		z-index: 1000;
	}

	.tab:last-child:not(:first-child) {
		margin-right: 0;
	}

	/* A dummy to fill the remaining space */
	.tab-view:after {
		content: "";
		display: block;
		height: 100%;
		float: left;
		flex-grow: 1;
		overflow-y: auto;
	}

	@media (max-width: 991px) {
		.tab:last-child {
			width: 100%;
		}

		.tab:not(:last-child),
		.tab:not(:nth-last-child(-n+1)) {
			display: none !important;
		}
	}

	@media (min-width: 992px) {
		.tab:not(:nth-last-child(-n+3)),
		.tab:not(:nth-last-child(3)) .pop-tab {
			display: none !important;
		}
	}
</style>
