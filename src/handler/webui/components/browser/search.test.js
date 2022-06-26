import { mount } from '@vue/test-utils';
import SearchBrowser from './search.vue';
import SearchResult from './search-result.vue';


function instance() {
	return mount(SearchBrowser, {
		props: {
			urlroot: '//nohost/',
			selectedPlayer: 'example',
		},
	});
}

function mockResults() {
	return [
		{track: {uri: 'test://1', artist: 'Meh', title: 'Foo', album: '1234'}, matches: {}},
		{track: {uri: 'test://2', artist: 'Meh', title: 'Bar', album: 'qwer'}, matches: {}},
		{track: {uri: 'test://3', artist: 'Meh', title: 'Baz', album: 'asdf'}, matches: {}},
	];
}

global.fetch = jest.fn(() => {
	return Promise.resolve({
		status: 200,
		json: () => Promise.resolve({tracks: mockResults()}),
	});
});

beforeEach(() => { fetch.mockClear(); });

test('input must trigger search', async () => {
	let wrapper = instance();
	wrapper.get('.search-input input').setValue('foo');
	await wrapper.vm.$nextTick();
	await wrapper.vm.$nextTick();

	expect(fetch.mock.calls.length).toBe(1);
	expect(wrapper.vm.results).toHaveLength(mockResults().length);
});

test('empty input must not trigger search', async () => {
	let wrapper = instance();
	wrapper.get('.search-input input').setValue('');
	await wrapper.vm.$nextTick();
	await wrapper.vm.$nextTick();

	expect(fetch.mock.calls.length).toBe(0);
	expect(wrapper.vm.results).toHaveLength(0);
});

test('search result must render', async () => {
	let wrapper = instance();
	wrapper.setData({'results': mockResults()});
	await wrapper.vm.$nextTick();

	let results = wrapper.findAllComponents(SearchResult);
	expect(results).toHaveLength(mockResults().length);
});

test('click on result must append to queue', async () => {
	let wrapper = instance();
	wrapper.setData({'results': mockResults()});
	await wrapper.vm.$nextTick();

	let result = wrapper.findComponent(SearchResult);
	result.trigger('click');
	expect(fetch.mock.calls.length).toBe(1);
	expect(fetch.mock.calls[0][0]).toBe('//nohost/data/player/example/playlist');
	expect(JSON.parse(fetch.mock.calls[0][1].body)).
		toStrictEqual({at: 'End', position: -1,tracks: ['test://1']});
});

test('click on album must trigger album selection', async () => {
	let wrapper = instance();
	wrapper.setData({'results': mockResults()});
	await wrapper.vm.$nextTick();

	let result = wrapper.findComponent(SearchResult);
	result.get('.track-album').trigger('click')
	expect(wrapper.emitted('show:album')).toBeTruthy();
});

test('click on empty album must not trigger album selection', async () => {
	let wrapper = instance();
	let results = mockResults();
	delete results[0].track.album;
	wrapper.setData({'results': results});
	await wrapper.vm.$nextTick();

	let result = wrapper.findComponent(SearchResult);
	result.get('.track-album').trigger('click')
	expect(wrapper.emitted('show:album')).toBeFalsy();
});
