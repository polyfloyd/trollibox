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
			try {
				let response = await fetch(url);
				if (!response.ok) {
					throw new Error('could not fetch track art');
				}
				this.blobUrl = URL.createObjectURL(await response.blob());
			} catch { }
		},
	},
});
