'use strict';

var TabView = Backbone.View.extend({
	tagName:   'div',
	className: 'tab-view',

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
		$tab.find('.do-pop-tab').on('click', (event) => {
			event.stopPropagation();
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
		return $tab;
	},

	popTab: function() {
		this.$('.tab:last-child').remove();
	},

	clearTabs: function() {
		this.$('.tab').remove();
	},

	escapeName: function(name) {
		return this.escapeNameTemplate({ name: name });
	},

	tabTemplate: _.template(`
		<div
			class="tab <%- name ? 'tab-name-'+name : '' %>"
			data-name="<%- name %>"></div>
	`),
	escapeNameTemplate: _.template('<%- name %>'),
});
