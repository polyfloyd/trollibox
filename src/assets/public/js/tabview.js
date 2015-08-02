'use strict';

var TabView = Backbone.View.extend({
	tagName:   'div',
	className: 'tab-view',

	initialize: function(args) {
		args || (args = {});
		this.maxTabs = args.maxTabs || 0;
	},

	/**
	 * Shows a tab (yes, really).
	 *
	 * If the tab is named and a tab with the same name already exists, it is
	 * substituted. In adittion, all tabs to the right are removed.
	 */
	pushTab: function(content, options) {
		var self = this;
		options || (options = {});

		var $tab = $(this.tabTemplate(options)).append(content);
		$tab.find('.do-pop-tab').on('click', function() {
			self.popTab();
		});

		var $old = options.name && this.$('.tab[data-name="'+options.name.replace(/"/g, '\\"')+'"]');
		if ($old.length) {
			$old.after($tab);
			$old.remove();
		} else {
			this.$el.append($tab);
		}

		$tab.find('~').remove();
		this.updateTabVisibility();

		return $tab;
	},

	popTab: function() {
		this.$('.tab:last-child').remove();
		this.updateTabVisibility();
	},

	clearTabs: function() {
		this.$('.tab').remove();
		this.updateTabVisibility();
	},

	updateTabVisibility: function() {
		if (this.maxTabs) {
			Array.prototype.forEach.call(this.$('.tab'), function($tab, i, arr) {
				$($tab).toggleAttr('hidden', i < arr.length - this.maxTabs);
			}, this);
		} else {
			this.$('.tab').toggleAttr('hidden', false);
		}
	},

	escapeName: function(name) {
		return this.escapeNameTemplate({ name: name });
	},

	tabTemplate: _.template(
		'<div '+
			'class="tab <%- name ? \'tab-name-\'+name : \'\' %>" '+
			'data-name="<%- name %>"></div>'
	),
	escapeNameTemplate: _.template('<%- name %>'),

});
