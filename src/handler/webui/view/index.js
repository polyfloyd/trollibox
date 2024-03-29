import 'bootstrap/dist/css/bootstrap.css';

import '../components/css/style.css';
import '../components/css/browser.css';
import '../components/css/track.css';

import { createApp } from 'vue';

import BrowserAlbums from '../components/browser/albums.vue';
import BrowserFiles from '../components/browser/files.vue';
import BrowserGenres from '../components/browser/genres.vue';
import BrowserQueuer from '../components/browser/queuer.vue';
import BrowserSearch from '../components/browser/search.vue';
import BrowserStreams from '../components/browser/streams.vue';
import Player from '../components/player.vue';
import SelectPlayer from '../components/select-player.vue';


let app = createApp({
	components: {
		BrowserAlbums,
		BrowserFiles,
		BrowserGenres,
		BrowserQueuer,
		BrowserSearch,
		BrowserStreams,
		Player,
		SelectPlayer,
	},
	data() {
		return {
			selectAlbumByTrack: null,
			...initialData,
		};
	},
	mounted() {
		document.body.addEventListener('keypress', event => {
			if (event.target != document.body) return;
			// Map number keys to views.
			let views = ['search', 'albums', 'genres', 'files', 'streams', 'queuer', 'player'];
			let i = Object.keys(views).indexOf((event.key - 1)+'');
			if (i != -1) {
				this.currentView = views[i];
			}
		});
		document.onkeydown = event => {
			if (event.ctrlKey && event.key == 'k') {
				event.preventDefault();
				this.currentView = 'search';
			}
		};
	},
	watch: {
		currentView(view) {
			this.showNavbar = false;

			let url = `${this.urlroot}player/${this.selectedPlayer}?view=${view}`;
			history.replaceState(null, document.title, url);
		},
	},
});
app.mount('#app');
