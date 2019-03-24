class FilterChangedEvent extends Event {
	constructor(name) {
		super(`change:${name}`);
		this.name = name;
	}
}

class FilterDB extends EventTarget {
	constructor(args) {
		super();
		this.filters = {};

		this._ev = new EventSource(`${URLROOT}data/filters/events`);
		this._ev.onopen = () => {
			// Reload all state to ensure that we are in sync.
			this._reload();
		};
		this._ev.addEventListener('filter:update', async event => {
			const name = JSON.parse(event.data).filter;
			this.filters[name] = await this._loadFilter(name);
			this.dispatchEvent(new FilterChangedEvent(name));
		});
		this._reload();
	}

	async store(name, filter) {
		const res = await fetch(`${URLROOT}data/filters/${name}`, {
			method: 'PUT',
			headers: {
				'Content-Type': 'application/json',
				// Trigger a an error response in JSON format.
				'X-Requested-With': 'fetch',
			},
			body: JSON.stringify({ filter }),
		});
		if (res.status == 400) {
			throw await res.json();
		} else if (res.status >= 400) {
			throw new Error('could not store filter');
		}
	}

	async remove(name) {
		const res = await fetch(`${URLROOT}data/filters/${name}`, {
			method: 'DELETE',
		});
		if (res.status >= 400) {
			throw new Error('could not remove filter');
		}
	}

	async _loadFilter(name) {
		const res = await fetch(`${URLROOT}data/filters/${name}`);
		if (res.status == 404) {
			return null;
		} else if (res.status >= 400) {
			throw new Error('could not load filter');
		}
		const { filter } = await res.json();
		return filter;
	}

	async _reload() {
		const res = await fetch(`${URLROOT}data/filters`);
		if (res.status >= 400) {
			throw new Error('could not list filters');
		}
		const { filters } = await res.json();
		for (const name of filters) {
			this.filters[name] = await this._loadFilter(name);
			this.dispatchEvent(new FilterChangedEvent(name));
		}
	}
}
