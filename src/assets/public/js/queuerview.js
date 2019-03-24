class QueuerView {
	constructor(filterdb) {
		this.filterdb = filterdb;
		this.$el = $('<div class="view queuer">');
		this.filterdb.addEventListener('change:queuer', () => {
			this.copyRules();
			this.render();
			this.removeRuleErrors();
		});
		this.copyRules();
		this.render();
	}

	copyRules() {
		const ft = this.filterdb.filters.queuer;
		if (!ft) {
			this.rules = [];
			return
		}
		// Clone to prevent modifying the array contained in the filterdb.
		this.rules = JSON.parse(JSON.stringify(ft.value.rules));
	}

	render() {
		this.$el.html(queuerViewTemplate());
		this.$el.find('.do-add-rule').on('click', () => this.doAddRule());
		this.$el.find('.queuer-rules').append(this.rules.map((rule, ruleIndex) => {
			const attr = this.ruleAttr(rule);
			const ops = queuerViewOps.filter(op => op.types.indexOf(attr.type) !== -1);

			const $el = $(queuerViewRuleTemplate({
				attrs:     queuerViewAttrs,
				ops:       ops,
				rule:      rule,
				renderVal: this.ruleAttr(rule).renderVal || (v => v),
			}));

			$el.find('.queuer-invert').on('change', event => {
				rule.invert = event.target.checked;
				this.updateRules();
			});

			$el.find('.queuer-attribute').on('change', event => {
				rule.attribute = $(event.target).val();
				const type = this.ruleAttr(rule).type;
				if (this.ruleOp(rule).types.indexOf(type) === -1) {
					rule.operation = 'equals';
				}
				try {
					if (type === 'int') QueuerView.stringToInt(rule.value)
				} catch {
					rule.value = 0;
				}
				this.updateRules();
			});

			$el.find('.queuer-operation').on('change', event => {
				rule.operation = $(event.target).val();
				this.updateRules();
			});

			$el.find('.queuer-value')[0].addEventListener('input', event => {
				$(event.target).addClass('modified');
			});
			$el.find('.queuer-value').on('change', event => {
				const $input = $(event.target)
				if (this.ruleAttr(rule).type === 'int') {
					const strVal = $input.val();
					try {
						rule.value = QueuerView.stringToInt(strVal);
					} catch {
						this.renderError({
							data: { index: ruleIndex },
							error: `"${strVal}" can not be interpreted as an integer`,
						});
						return;
					}
				} else {
					rule.value = $input.val();
				}

				$input.removeClass('modified');
				this.updateRules();
			});
			$el.find('.do-remove').on('click', () => {
				this.rules.splice(ruleIndex, 1);
				this.updateRules();
			});
			return $el;
		}));
	}

	renderError(err) {
		if (err.data && typeof err.data.index == 'number') {
			const $li = this.$el.find('.queuer-rules > li:nth-child('+(err.data.index+1)+') .queuer-value');
			$li.tooltip({
				title:    err.error,
				template: queuerViewRuleErrorTemplate(),
				trigger:  'manual',
			}).tooltip('show');
			return;
		}
		this.$el.find('.error-message').text(err.message);
	}

	removeRuleErrors() {
		this.$el.find('.queuer-rules > li .queuer-value').tooltip('destroy');
		this.$el.find('.error-message').empty();
	}

	ruleAttr(rule) {
		return queuerViewAttrs.filter(attr => attr.name === rule.attribute)[0];
	}

	ruleOp(rule) {
		return queuerViewOps.filter(op => op.name === rule.operation)[0];
	}

	async updateRules() {
		this.removeRuleErrors();
		try {
			await this.filterdb.store('queuer', {
				type:  'ruled',
				value: { rules: this.rules },
			});
		} catch (err) {
			this.renderError(err);
		}
	}

	doAddRule() {
		this.rules.push({
			attribute: 'artist',
			invert:    false,
			operation: 'contains',
			value:     '',
		});
		this.updateRules();
	}

	static stringToInt(str) {
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

		throw new Error('invalid integer format');
	}

	// XXX: Transitional
	on(event, handler) { }
}

const queuerViewAttrs = [
	{
		name: 'uri',
		type: 'string',
	},
	{
		name: 'artist',
		type: 'string',
	},
	{
		name: 'title',
		type: 'string',
	},
	{
		name: 'genre',
		type: 'string',
	},
	{
		name: 'album',
		type: 'string',
	},
	{
		name: 'albumartist',
		type: 'string',
	},
	{
		name: 'albumtrack',
		type: 'string',
	},
	{
		name: 'albumdisc',
		type: 'string',
	},
	{
		name:      'duration',
		type:      'int',
		renderVal: durationToString,
	},
];
const queuerViewOps = [
	{
		name: 'contains',
		types: [ 'string' ],
	},
	{
		name: 'equals',
		types: [ 'string', 'int' ],
	},
	{
		name: 'greater',
		types: [ 'string', 'int' ],
	},
	{
		name: 'less',
		types: [ 'string', 'int' ],
	},
	{
		name: 'matches',
		types: [ 'string' ],
	},
];
const queuerViewTemplate = _.template(`
	<div>
		<h2>AutoQueuer Rules</h2>
		<p class="error-message"></p>
		<ul class="queuer-rules"></ul>
		<button class="glyphicon glyphicon-plus do-add-rule"></ul>
	</div>
`);
const queuerViewRuleTemplate = _.template(`
	<li class="form-inline">
		<div class="input-group">
			<label class="input-group-addon queuer-invert">
				<input type="checkbox" <%= rule.invert ? 'checked' : '' %> />
				if <span>not</span>
			</label>

			<select class="form-control queuer-attribute">
			<% attrs.forEach(function(attr) { %>
				<option
					value="<%= attr.name %>"
					<%= rule.attribute === attr.name ? 'selected' : '' %>
					><%- attr.name %></option>
			<% }) %>
			</select>
		</div>

		<select class="form-control queuer-operation">
		<% ops.forEach(function(op) { %>
			<option
				value="<%= op.name %>"
				<%= rule.operation === op.name ? 'selected' : '' %>
				><%- op.name %></option>
		<% }) %>
		</select>

		<div class="input-group">
			<input
				class="form-control queuer-value"
				type="text"
				placeholder="value"
				value="<%- renderVal(rule.value) %>" />
			<span class="input-group-addon field-modified">*</span>
			<span class="input-group-addon addon-and">and</span>
		</div>

		<button class="glyphicon glyphicon-remove do-remove"></button>
	</li>
`);
const queuerViewRuleErrorTemplate = _.template(`
	<div class="tooltip rule-error" role="tooltip">
		<div class="glyphicon glyphicon-warning-sign"></div>
		<div class="tooltip-inner"></div>
		<div class="tooltip-arrow"></div>
	</div>
`);
