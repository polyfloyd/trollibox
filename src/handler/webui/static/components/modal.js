Vue.component('modal', {
	template: `
		<div class="modal" @click="close">
			<slot></slot>
		</div>
	`,
	methods: {
		close: function() {
			this.$emit('close');
		},
	},
});
