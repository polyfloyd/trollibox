'use strict';

var QueuerView = Backbone.View.extend({
	ATTRS: [
		{
			name: 'id',
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
			name: 'duration',
			type: 'int',
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

	events: {
		'click .do-add-rule': 'doAddRule',
	},

	initialize: function() {
		this.listenTo(this.model, 'change:queuerules', function(obj, value, options) {
			if (!options.queuerViewNoRender) {
				this.copyRules();
				this.render();
			}
			this.removeRuleErrors();
		});
		this.listenTo(this.model, 'error:queuerules', this.renderError);
		this.copyRules();
		this.render();
	},

	copyRules: function() {
		// Map using Object.create to prevent modifying the array contained in the model.
		this.rules = this.model.get('queuerules').map(function(rule) {
			var mutRule = {};
			for (var k in rule) mutRule[k] = rule[k];
			return mutRule;
		});
	},

	render: function() {
		var self = this;

		this.$el.html(this.template());
		this.$('.queuer-rules').append(this.rules.map(function(rule, ruleIndex) {
			var attr = this.ruleAttr(rule);
			var ops = this.OP.filter(function(op) {
				return op.types.indexOf(attr.type) !== -1;
			});

			var $el = $(this.ruleTemplate({
				attrs: this.ATTRS,
				ops:   ops,
				rule:  rule,
			}));

			$el.find('.queuer-invert').on('change', function() {
				rule.invert = $(this).find('change').prop('checked');
				self.updateRules();
			});

			$el.find('.queuer-attribute').on('change', function() {
				rule.attribute = $(this).val();
				var type = self.ruleAttr(rule).type;
				if (self.ruleOp(rule).types.indexOf(type) === -1) {
					rule.operation = 'equals';
				}
				if (type === 'int' && Number.isNaN(self.stringToInt(rule.value))) {
					rule.value = 0;
				}
				self.updateRules();
			});

			$el.find('.queuer-operation').on('change', function() {
				rule.operation = $(this).val();
				self.updateRules();
			});

			$el.find('.queuer-value').on('input', function() {
				if (self.ruleAttr(rule).type === 'int') {
					var val = self.stringToInt($(this).val());
					if (!Number.isNaN(val)) {
						rule.value = val;
					} else {
						self.trigger('error', new Error(val+' can not be interpreted as an integer'));
						return;
					}
				} else {
					rule.value = $(this).val();
				}

				// Set the noRender flag to preserve focus on the inputfield.
				self.updateRules(true);
			});
			$el.find('.do-remove').on('click', function() {
				self.rules.splice(ruleIndex, 1);
				self.updateRules();
			});
			return $el;
		}, this));
	},

	renderError: function(err) {
		var $li = this.$('.queuer-rules > li:nth-child('+(err.ruleindex+1)+') .queuer-value');
		$li.tooltip({
			title:    err.message,
			template: this.ruleErrorTemplate(),
			trigger:  'manual',
		}).tooltip('show');
	},

	removeRuleErrors: function() {
		this.$('.queuer-rules > li .queuer-value').tooltip('destroy');
	},

	stringToInt: function(str) {
		if (str.match(/^(\d+:)?(\d{1,2}:)?\d{1,2}$/)) { // [[hh:]mm:]ss time
			return str.match(/(\d+)/g).reduce(function(time, num, i, arr) {
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
		return this.ATTRS.filter(function(attr) {
			return attr.name === rule.attribute;
		})[0];
	},

	ruleOp: function(rule) {
		return this.OP.filter(function(op) {
			return op.name === rule.operation;
		})[0];
	},

	updateRules: function(noRender) {
		this.model.set('queuerules', this.rules, {
			silent: true,
		});
		this.model.trigger('change:queuerules', this.model, this.rules, {
			queuerViewNoRender: !!noRender,
		});
	},

	doAddRule: function() {
		this.model.addDefaultQueueRule();
	},

	template: _.template(
		'<div class="queuer">'+
			'<h2>Queue Rules</h2>'+
			'<ul class="queuer-rules"></ul>'+
			'<button class="glyphicon glyphicon-plus do-add-rule"></ul>'+
		'</div>'
	),
	ruleTemplate: _.template(
		'<li class="form-inline">'+
			'<div class="input-group">'+
				'<label class="input-group-addon queuer-invert">'+
					'<input type="checkbox" <%= rule.invert ? \'checked\' : \'\' %> />'+
					'if <span>not</span>'+
				'</label>'+

				'<select class="form-control queuer-attribute">'+
				'<% attrs.forEach(function(attr) { %>'+
					'<option '+
						'value="<%= attr.name %>" '+
						'<%= rule.attribute === attr.name ? \'selected\' : \'\' %>'+
						'><%- attr.name %></option>'+
				'<% }) %>'+
				'</select>'+
			'</div>'+

			'<select class="form-control queuer-operation">'+
			'<% ops.forEach(function(op) { %>'+
				'<option '+
					'value="<%= op.name %>" '+
					'<%= rule.operation === op.name ? \'selected\' : \'\' %>'+
					'><%- op.name %></option>'+
			'<% }) %>'+
			'</select>'+

			'<div class="input-group">'+
				'<input '+
					'class="form-control queuer-value" '+
					'type="text" '+
					'placeholder="value" '+
					'value="<%- rule.value %>" />'+
				'<span class="input-group-addon addon-and">and</span>'+
			'</div>'+

			'<button class="glyphicon glyphicon-remove do-remove"></button>'+
		'</li>'
	),
	ruleErrorTemplate: _.template(
		'<div class="tooltip rule-error" role="tooltip">'+
			'<div class="glyphicon glyphicon-warning-sign"></div>'+
			'<div class="tooltip-inner"></div>'+
			'<div class="tooltip-arrow"></div>'+
		'</div>'
	),
});
