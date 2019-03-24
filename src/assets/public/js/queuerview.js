'use strict';

var QueuerView = Backbone.View.extend({
	ATTRS: [
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
	],
	OP: [
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
	],

	tagName:   'div',
	className: 'view queuer',

	events: {
		'click .do-add-rule': 'doAddRule',
	},

	initialize: function() {
		this.model.addEventListener('change:queuer', () => {
			this.copyRules();
			this.render();
			this.removeRuleErrors();
		});
		this.listenTo(this, 'error', this.renderError);
		this.copyRules();
		this.render();
	},

	copyRules: function() {
		var ft = this.model.filters.queuer;
		if (!ft) {
			this.rules = [];
			return
		}

		// Map to prevent modifying the array contained in the model.
		this.rules = ft.value.rules.map((rule) => {
			var mutRule = {};
			for (var k in rule) mutRule[k] = rule[k];
			return mutRule;
		});
	},

	render: function() {
		this.$el.html(this.template());
		this.$('.queuer-rules').append(this.rules.map((rule, ruleIndex) => {
			var attr = this.ruleAttr(rule);
			var ops = this.OP.filter((op) => {
				return op.types.indexOf(attr.type) !== -1;
			});

			var $el = $(this.ruleTemplate({
				attrs:     this.ATTRS,
				ops:       ops,
				rule:      rule,
				renderVal: this.ruleAttr(rule).renderVal || (v => v),
			}));

			$el.find('.queuer-invert')[0].addEventListener('change', event => {
				rule.invert = event.target.checked;
				this.updateRules();
			});

			$el.find('.queuer-attribute').on('change', (event) => {
				rule.attribute = $(event.target).val();
				var type = this.ruleAttr(rule).type;
				if (this.ruleOp(rule).types.indexOf(type) === -1) {
					rule.operation = 'equals';
				}
				if (type === 'int' && Number.isNaN(this.stringToInt(rule.value))) {
					rule.value = 0;
				}
				this.updateRules();
			});

			$el.find('.queuer-operation').on('change', event => {
				rule.operation = $(event.target).val();
				this.updateRules();
			});

			$el.find('.queuer-value')[0].addEventListener('input', (event) => {
				$(event.target).addClass('modified');
			});
			$el.find('.queuer-value').on('change', (event) => {
				var $input = $(event.target)
				if (this.ruleAttr(rule).type === 'int') {
					var strVal = $input.val();
					var val = this.stringToInt(strVal);
					if (Number.isNaN(val)) {
						var err = new Error('"'+strVal+'" can not be interpreted as an integer')
						err.data = { index: ruleIndex, };
						this.trigger('error', err);
						return;
					}
					rule.value = val;
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
		}, this));
	},

	renderError: function(err) {
		if (err.data && typeof err.data.index == 'number') {
			var $li = this.$('.queuer-rules > li:nth-child('+(err.data.index+1)+') .queuer-value');
			$li.tooltip({
				title:    err.error,
				template: this.ruleErrorTemplate(),
				trigger:  'manual',
			}).tooltip('show');
		} else {
			this.$('.error-message').text(err.message);
		}
	},

	removeRuleErrors: function() {
		this.$('.queuer-rules > li .queuer-value').tooltip('destroy');
		this.$('.error-message').empty();
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

		return NaN;
	},

	ruleAttr: function(rule) {
		return this.ATTRS.filter((attr) => {
			return attr.name === rule.attribute;
		})[0];
	},

	ruleOp: function(rule) {
		return this.OP.filter((op) => {
			return op.name === rule.operation;
		})[0];
	},

	updateRules: function() {
		this.removeRuleErrors();
		this.model.store('queuer', {
			type:  'ruled',
			value: { rules: this.rules },
		})
			.catch(err => {
				this.trigger('error', err);
			});
	},

	doAddRule: function() {
		this.rules.push({
			attribute: 'artist',
			invert:    false,
			operation: 'contains',
			value:     '',
		});
		this.updateRules();
	},

	template: _.template(`
		<div>
			<h2>AutoQueuer Rules</h2>
			<p class="error-message"></p>
			<ul class="queuer-rules"></ul>
			<button class="glyphicon glyphicon-plus do-add-rule"></ul>
		</div>
	`),
	ruleTemplate: _.template(`
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
	`),
	ruleErrorTemplate: _.template(`
		<div class="tooltip rule-error" role="tooltip">
			<div class="glyphicon glyphicon-warning-sign"></div>
			<div class="tooltip-inner"></div>
			<div class="tooltip-arrow"></div>
		</div>
	`),
});
