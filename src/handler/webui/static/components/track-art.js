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
		this.load();
	},
	watch: {
		track: function() {
			this.blobUrl = null;
			this.load();
		},
	},
	methods: {
		load: async function() {
			if (!this.track) return;

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
