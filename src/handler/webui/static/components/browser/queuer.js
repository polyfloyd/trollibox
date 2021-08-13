const queuerViewAttrs = [
	{name: 'uri', type: 'string'},
	{name: 'artist', type: 'string'},
	{name: 'title', type: 'string'},
	{name: 'genre', type: 'string'},
	{name: 'album', type: 'string'},
	{name: 'albumartist', type: 'string'},
	{name: 'albumtrack', type: 'string'},
	{name: 'albumdisc', type: 'string'},
	{name: 'duration', type: 'int'},
];
const queuerViewOps = [
	{name: 'contains', types: ['string']},
	{name: 'equals', types: ['string', 'int']},
	{name: 'greater', types: ['string', 'int']},
	{name: 'less', types: ['string', 'int']},
	{name: 'matches', types: ['string']},
];

Vue.component('browser-queuer', {
	mixins: [TrackMixin],
	props: {
		urlroot: {required: true, type: String},
	},
	data: function() {
		return {
			rules: [],
			ruleErrors: {},
			error: '',
		};
	},
	template: `
		<div class="browser-queuer">
			<h2>AutoQueuer Rules</h2>

			<div>
				<p class="error-message">{{ error }}</p>

				<ul class="queuer-rules">
					<li v-for="(rule, i) in rules" class="form-inline">
						<div class="input-group">
							<label class="input-group-addon queuer-invert">
								<input type="checkbox" v-model="rule.invert" />
								if <span>not</span>
							</label>
							<select class="form-control queuer-attribute"
								:value="rule.attribute" @change="setRuleAttribute(i, $event.target.value)">
								<option v-for="attr in attrs" :value="attr.name">{{ attr.name }}</option>
							</select>
						</div><!--
						--><select class="form-control queuer-operation"
							:value="rule.operation" @change="setRuleOperation(i, $event.target.value)">
							<option v-for="op in opsForRule(rule)" :value="op.name">{{ op.name }}</option>
						</select><!--
						--><div class="input-group">
							<input v-if="attrForRule(rule).type == 'string'"
								:value="rule.value"
								@change="setRuleStringValue(i, $event.target.value)"
								class="form-control queuer-value" type="text" placeholder="value" />
							<input v-else-if="attrForRule(rule).type == 'int'"
								:value="durationToString(rule.value)"
								@keydown.enter="setRuleIntValue(i, $event.target.value)"
								@focus="setRuleIntValue(i, $event.target.value)"
								@blur="setRuleIntValue(i, $event.target.value)"
								class="form-control queuer-value" type="text" placeholder="value" />
							<span class="input-group-addon addon-and">and</span>
						</div>
						<button class="glyphicon glyphicon-remove do-remove" @click="removeRule(i)"></button>

						<span v-if="ruleErrors[i]" class="rule-error">
							<div class="glyphicon glyphicon-warning-sign"></div>
							{{ ruleErrors[i] }}
						</span>
					</li>
				</ul>

				<button class="glyphicon glyphicon-plus" @click="addRule"></button>
			</div>
		</div>
	`,
	created: function() {
		this._ev = new EventSource(`${this.urlroot}data/filters/events`);
		this._ev.onopen = () => {
			// Reload all state to ensure that we are in sync.
			this.reload();
		};
		this._ev.addEventListener('filter:update', async event => {
			let name = JSON.parse(event.data).filter;
			if (name != this.filterName) return;
			await this.reload();
		});
		this.reload();
	},
	destroyed: function() {
		this._ev.close();
	},
	computed: {
		filterName: function() { return 'queuer'; },
		attrs: function() { return queuerViewAttrs; },
		ops: function() { return queuerViewOps; },
	},
	methods: {
		addRule: function() {
			this.rules.push({
				attribute: 'artist',
				invert:    false,
				operation: 'contains',
				value:     '',
			});
			this.update();
		},
		removeRule: function(ruleIndex) {
			this.rules.splice(ruleIndex, 1);
			this.update();
		},
		setRuleAttribute: function(ruleIndex, attr) {
			this.rules[ruleIndex].attribute = attr;
			this.update();
		},
		setRuleOperation: function(ruleIndex, op) {
			this.rules[ruleIndex].operation = op;
			this.update();
		},
		setRuleStringValue: function(ruleIndex, value) {
			this.rules[ruleIndex].value = value;
			this.update();
		},
		setRuleIntValue: function(ruleIndex, value) {
			try {
				this.rules[ruleIndex].value = this.stringToInt(value);
				this.setRuleError(ruleIndex, null);
				this.update();
			} catch (err) {
				this.setRuleError(ruleIndex, err);
			}
		},
		setRuleError: function(ruleIndex, err) {
			let ruleErrors = {...this.ruleErrors};
			if (err) ruleErrors[ruleIndex] = ''+err;
			else delete ruleErrors[ruleIndex];
			this.ruleErrors = ruleErrors;
		},

		reload: async function() {
			let response = await fetch(`${this.urlroot}data/filters/${this.filterName}`);
			if (response.status == 404) {
				console.warning(`Queuer filter ${this.filterName} does not exist on the server`);
				this.rules = [];
				return;
			} else if (!response.ok) {
				throw new Error('could not load filter');
			}
			let { filter } = await response.json();
			if (filter.type != 'ruled') {
				console.warning(`Unexpected type for queuer filter: ${filter.type}`);
				this.rules = [];
				return;
			}
			this.rules = filter.rules;
		},
		update: async function() {
			let response = await fetch(`${this.urlroot}data/filters/${this.filterName}`, {
				method: 'PUT',
				headers: {
					'Content-Type': 'application/json',
					// Trigger an error response in JSON format.
					'X-Requested-With': 'fetch',
				},
				body: JSON.stringify({filter: {type: 'ruled', rules: this.rules}}),
			});
			if (response.status == 400) {
				let err = await response.json();
				if (err.data && typeof err.data.index == 'number') {
					this.setRuleError(err.data.index, err.error);
				} else {
					this.error = ''+err.error;
				}
			} else if (!response.ok) {
				throw new Error('Could not store filter');
			} else {
				this.ruleErrors = {};
				this.error = '';
			}
		},

		attrForRule: function(rule) {
			let a = this.attrs.find(a => a.name == rule.attribute);
			if (!a) throw new Error(`no such attr: ${rule.attribute}`);
			return a;
		},
		opsForRule: function(rule) {
			let attr = this.attrForRule(rule);
			return this.ops.filter(op => op.types.some(t => t == attr.type));
		},

		stringToInt: function(str) {
			if (str.match(/^(\d+:)?(\d{1,2}:)?\d{1,2}$/)) { // [[hh:]mm:]ss time
				return str.match(/(\d+)/g).reduce((time, num, i, arr) => {
					return time + Math.pow(60, (arr.length - i - 1)) * parseInt(num, 10);
				}, 0);
			} else if (str.match(/^0b[01]+$/)) { // Binary
				return parseInt(str.match(/^0b([01]+)$/)[1], 2);
			} else if (str.match(/^0[0-7]+$/)) { // Octal
				return parseInt(str.match(/^0([0-7]+)$/)[1], 8);
			} else if (str.match(/^\d+$/)) { // Decimal
				return parseInt(str, 10);
			} else if (str.match(/^(0x)?[0-9a-f]+$/i)) { // Hexadecimal
				return parseInt(str.match(/^(0x)?0*([0-9a-f]+)$/i)[2], 16);
			}
			throw new Error('Bad number format');
		},
	},
});
