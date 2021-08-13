let trackArtImageCache = new Map();

Vue.component('track-art', {
	mixins: [ApiMixin],
	props: {
		track: Object,
	},
	data: function() {
		return {blobUrl: null};
	},
	template: `
		<div class="track-art" :class="{placeholder: !blobUrl}"
			:style="{backgroundImage: blobUrl && 'url(\\''+blobUrl+'\\')'}"></div>
	`,
	mounted: function() {
		let elem = this.$el;
		while (elem.parentElement) {
			let w = elem.addEventListener('scroll', this.load)
			elem = elem.parentElement;
		}

		setTimeout(this.load, 100);
	},
	beforeDestroy: function() {
		let elem = this.$el;
		while (elem.parentElement) {
			let w = elem.removeEventListener('scroll', this.load)
			elem = elem.parentElement;
		}
	},
	watch: {
		track: function() {
			this.blobUrl = null;
			this.load();
		},
	},
	methods: {
		visible: function() {
			// An element is visible if a part of its bounding box is inside
			// the bounding box of all its parents.
			let elem = this.$el;
			let rect = elem.getBoundingClientRect();
			while (elem.parentElement) {
				let parentRect = elem.parentElement.getBoundingClientRect();
				elem = elem.parentElement;
				rect = {
					top: Math.max(rect.top, parentRect.top),
					left: Math.max(rect.left, parentRect.left),
					right: Math.min(rect.right, parentRect.right),
					bottom: Math.min(rect.bottom, parentRect.bottom),
				};
			}
			return (rect.bottom - rect.top) > 0 && (rect.right - rect.left) > 0;
		},
		load: async function() {
			// Do not attempt to load if there is no track or an image has
			// already been loaded.
			if (!this.track || this.blobUrl) return;
			// Do not load if the element is not visible to save resources.
			if (!this.visible()) return;

			let url = `${this.urlroot}data/player/${this.selectedPlayer}/tracks/art?track=${encodeURIComponent(this.track.uri)}`;

			if (!trackArtImageCache.has(url)) {
				try {
					let promise = fetch(url)
						.then(async response => {
							if (!response.ok) return null;
							return URL.createObjectURL(await response.blob());
						});
					trackArtImageCache.set(url, promise);
				} catch (e) {
					trackArtImageCache.delete(url);
				}
			}

			if (trackArtImageCache.has(url)) {
				this.blobUrl = await trackArtImageCache.get(url);
			}
		},
	},
});
