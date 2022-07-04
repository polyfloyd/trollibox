<template>
	<div class="rule-filter">
		<ul class="rules">
			<li v-for="(rule, i) in modelValue.rules" class="form-inline">
				<div class="input-group">
					<label class="input-group-addon rule-invert">
						<input type="checkbox" :checked="rule.invert" @input="toggleInvert(i)" />
						if <span>not</span>
					</label>
					<select class="form-control rule-attribute"
						:value="rule.attribute" @change="setAttribute(i, $event.target.value)">
						<option v-for="attr in attrs" :value="attr.name">{{ attr.name }}</option>
					</select>
				</div><!--
				--><select class="form-control rule-operation"
					:value="rule.operation" @change="setOperation(i, $event.target.value)">
					<option v-for="op in opsForRule(rule)" :value="op.name">{{ op.name }}</option>
				</select><!--
				--><div class="input-group">
					<input v-if="attrForRule(rule).type == 'string'"
						:value="rule.value"
						@change="setValue(i, $event.target.value)"
						class="form-control rule-value" type="text" placeholder="value" />
					<input v-else-if="attrForRule(rule).type == 'number'"
						:value="durationToString(rule.value)"
						@keydown.enter="setIntValue(i, $event.target.value)"
						@focus="setIntValue(i, $event.target.value)"
						@blur="setIntValue(i, $event.target.value)"
						class="form-control rule-value" type="text" placeholder="value" />
					<span class="input-group-addon addon-and">and</span>
				</div>
				<button class="glyphicon glyphicon-remove do-remove" @click="removeRule(i)"></button>
			</li>
		</ul>

		<button class="glyphicon glyphicon-plus" @click="addRule"></button>
	</div>
</template>

<script>
	import TrackMixin from '../mixins/track.js';

	let ruleAttributes = [
		{name: 'uri', type: 'string', init: ''},
		{name: 'artist', type: 'string', init: ''},
		{name: 'title', type: 'string', init: ''},
		{name: 'genre', type: 'string', init: ''},
		{name: 'album', type: 'string', init: ''},
		{name: 'albumartist', type: 'string', init: ''},
		{name: 'albumtrack', type: 'string', init: ''},
		{name: 'albumdisc', type: 'string', init: ''},
		{name: 'duration', type: 'number', init: 0},
	];
	let ruleOperations = [
		{name: 'contains', types: ['string']},
		{name: 'equals', types: ['string', 'number']},
		{name: 'greater', types: ['string', 'number']},
		{name: 'less', types: ['string', 'number']},
		{name: 'matches', types: ['string']},
	];

	function validateRule(rule) {
		return ruleAttributes.some(attr => attr.name == rule.attribute)
			&& ruleOperations.some(op => op.name == rule.operation)
			&& typeof rule.invert == 'boolean';
	}

	export default {
		mixins: [TrackMixin],
		props: {
			modelValue: {requred: true, type: Object,
				validate: v => v.type == 'ruled' && v.rules.every(validateRule)},
		},
		emits: ['update:modelValue'],
		computed: {
			attrs() { return ruleAttributes; },
			ops() { return ruleOperations; },
		},
		methods: {
			addRule() {
				this.modelValue.rules.push({
					attribute: 'artist',
					invert:    false,
					operation: 'contains',
					value:     '',
				});
				this.$emit('update:modelValue', {...this.modelValue});
			},
			removeRule(ruleIndex) {
				this.modelValue.rules.splice(ruleIndex, 1);
				this.$emit('update:modelValue', {...this.modelValue});
			},
			toggleInvert(ruleIndex) {
				let rule = this.modelValue.rules[ruleIndex];
				rule.invert = !!(rule.invert ^ true);
				this.modelValue.rules[ruleIndex] = rule;
				this.$emit('update:modelValue', {...this.modelValue});
			},
			setAttribute(ruleIndex, attribute) {
				this.modelValue.rules[ruleIndex] =
					this.ensureValid({...this.modelValue.rules[ruleIndex], attribute});
				this.$emit('update:modelValue', {...this.modelValue});
			},
			setOperation(ruleIndex, operation) {
				this.modelValue.rules[ruleIndex] =
					this.ensureValid({...this.modelValue.rules[ruleIndex], operation});
				this.$emit('update:modelValue', {...this.modelValue});
			},
			setValue(ruleIndex, value) {
				this.modelValue.rules[ruleIndex].value = value;
				this.$emit('update:modelValue', {...this.modelValue});
			},
			setIntValue(ruleIndex, value) {
				try {
					this.setValue(ruleIndex, this.stringToInt(value));
				} catch (err) {
					this.setValue(ruleIndex, 0);
				}
			},

			ensureValid(rule) {
				let attr = this.attrForRule(rule);
				let ops = this.opsForRule(rule);
				if (!ops.some(v => v.name == rule.operation)) {
					rule.operation = ops[0].name;
				}
				if (!!rule.value || typeof rule.value != attr.type) {
					rule.value = attr.init;
				}
				return rule;
			},

			attrForRule(rule) {
				return ruleAttributes.find(a => a.name == rule.attribute);
			},
			opsForRule(rule) {
				let attr = this.attrForRule(rule);
				return ruleOperations.filter(op => op.types.some(t => t == attr.type));
			},

			stringToInt(str) {
				if (str.match(/^(\d+:)?(\d{1,2}:)?\d{1,2}$/)) { // [[hh:]mm:]ss time
					return str.match(/(\d+)/g).reduce((time, num, i, arr) => {
						return time + Math.pow(60, (arr.length - i - 1)) * parseInt(num, 10);
					}, 0);
				} else if (str.match(/^\d+$/)) { // Decimal
					return parseInt(str, 10);
				}
				throw new Error('Bad number format');
			},
		},
	}
</script>

<style scoped>
	.rules {
		padding: 0;
		list-style-type: none;
	}

	.rules .rule-invert {
		cursor: pointer;
		-webkit-user-select: none;
		-moz-user-select: none;
		user-select: none;
	}

	.rules .rule-invert input {
		display: none;
	}

	.rules .rule-invert span {
		color: var(--color-bg-elem);
	}

	.rules .rule-invert input:not(:checked) ~ span {
		opacity: 0;
	}

	.rules li:last-child .input-group-addon.addon-and {
		display: none;
	}

	.rule-filter button.glyphicon {
		font-size: 20px;
		border: none;
		color: var(--color-accent);
		background: none;
	}

	.rules li .do-remove {
		margin-left: 8px;
		vertical-align: middle;
	}

	.rules .rule-error .glyphicon {
		color: #f00;
	}
</style>
