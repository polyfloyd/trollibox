class TabView {
	constructor() {
		this.$el = $('<div class="tab-view">');
	}

	/**
	 * Shows a tab.
	 *
	 * If the tab is named and a tab with the same name already exists, it is
	 * substituted. In adittion, all tabs to the right are removed.
	 */
	pushTab(content, options) {
		var self = this;
		options || (options = {});

		var $tab = $(TabView.tabTemplate(options)).append(content);
		$tab.find('.do-pop-tab').on('click', (event) => {
			event.stopPropagation();
			self.popTab();
		});

		var $old = options.name && this.$el.find('.tab[data-name="'+options.name.replace(/"/g, '\\"')+'"]');
		if ($old.length) {
			$old.after($tab);
			$old.remove();
		} else {
			this.$el.append($tab);
		}

		$tab.find('~').remove();
		return $tab;
	}

	popTab() {
		this.$el.find('.tab:last-child').remove();
	}

	clearTabs() {
		this.$el.find('.tab').remove();
	}

	escapeName(name) {
		return TabView.escapeNameTemplate({ name: name });
	}
}

TabView.tabTemplate = _.template(`
	<div
		class="tab <%- name ? 'tab-name-'+name : '' %>"
		data-name="<%- name %>"></div>
`);
TabView.escapeNameTemplate = _.template('<%- name %>');
