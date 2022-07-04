import { mount } from '@vue/test-utils';
import RuleFilter from './rule-filter.vue';


function instance(modelValue) {
	return mount(RuleFilter, {
		props: {
			modelValue,
		},
	});
}

test('empty input must show nothing', () => {
	let wrapper = instance({type: 'ruled', rules: []});
	expect(wrapper.findAll('.rules > li')).toHaveLength(0);
});

test('rules must render', () => {
	let rules = [
		{attribute: 'artist', operation: 'contains', value: 'foo', invert: false},
		{attribute: 'title', operation: 'contains', value: 'bar', invert: false},
	];
	let wrapper = instance({type: 'ruled', rules});
	expect(wrapper.findAll('.rules > li')).toHaveLength(rules.length);
});

test('a rule must be added when the respective button is clicked', async () => {
	let wrapper = instance({type: 'ruled', rules: []});
	expect(wrapper.findAll('.rules > li')).toHaveLength(0);

	wrapper.get('button.glyphicon-plus').trigger('click');
	await wrapper.vm.$nextTick();
	expect(wrapper.findAll('.rules > li')).toHaveLength(1);
	expect(wrapper.emitted('update:modelValue')).toBeTruthy();
});

test('a rule must be removed when the respective button is clicked', async () => {
	let rules = [
		{attribute: 'artist', operation: 'contains', value: 'foo', invert: false},
		{attribute: 'title', operation: 'contains', value: 'bar', invert: false},
		{attribute: 'album', operation: 'contains', value: 'baz', invert: false},
	];
	let wrapper = instance({type: 'ruled', rules});

	wrapper.findAll('li > button.do-remove')[1].trigger('click');
	await wrapper.vm.$nextTick();
	expect(wrapper.findAll('.rules > li')).toHaveLength(2);
	expect(wrapper.emitted('update:modelValue')[0][0].rules.map(r => r.attribute))
		.toStrictEqual(['artist', 'album']);
});

test('change from number to text value', async () => {
	let rules = [{attribute: 'duration', operation: 'less', value: 600, invert: false}];
	let wrapper = instance({type: 'ruled', rules});

	wrapper.get('.rule-attribute').setValue('artist');
	await wrapper.vm.$nextTick();
	expect(wrapper.get('.rule-operation').element.value).toBe('less');
	expect(wrapper.get('.rule-value').element.value).toBe('');
});

test('change from number to text value', async () => {
	let rules = [{attribute: 'duration', operation: 'less', value: 600, invert: false}];
	let wrapper = instance({type: 'ruled', rules});

	wrapper.get('.rule-attribute').setValue('artist');
	await wrapper.vm.$nextTick();
	expect(wrapper.get('.rule-operation').element.value).toBe('less');
	expect(wrapper.get('.rule-value').element.value).toBe('');
});

test('changing from text to number attribute with incompatible op', async () => {
	let rules = [{attribute: 'artist', operation: 'contains', value: 'foo', invert: false}];
	let wrapper = instance({type: 'ruled', rules});

	wrapper.get('.rule-attribute').setValue('duration');
	await wrapper.vm.$nextTick();
	expect(wrapper.get('.rule-operation').element.value).toBe('equals');
	expect(wrapper.get('.rule-value').element.value).toBe('00:00');
});

test('altering rule attribute must trigger update', async () => {
	let rules = [{attribute: 'artist', operation: 'contains', value: 'foo', invert: false}];
	let wrapper = instance({type: 'ruled', rules});

	let attributeSelect = wrapper.get('.rule-attribute');
	expect(attributeSelect.element.value).toBe('artist');
	attributeSelect.setValue('title');
	await wrapper.vm.$nextTick();
	expect(wrapper.emitted('update:modelValue')).toBeTruthy();
});

test('altering rule operation must trigger update', async () => {
	let rules = [{attribute: 'artist', operation: 'contains', value: 'foo', invert: false}];
	let wrapper = instance({type: 'ruled', rules});

	let operationSelect = wrapper.get('.rule-operation');
	expect(operationSelect.element.value).toBe('contains');
	operationSelect.setValue('equals');
	await wrapper.vm.$nextTick();
	expect(wrapper.emitted('update:modelValue')).toBeTruthy();
});

test('altering rule text value must trigger update', async () => {
	let rules = [{attribute: 'artist', operation: 'contains', value: 'foo', invert: false}];
	let wrapper = instance({type: 'ruled', rules});

	let valueInput = wrapper.get('.rule-value');
	expect(valueInput.element.value).toBe('foo');
	valueInput.setValue('bar');
	await wrapper.vm.$nextTick();
	expect(wrapper.emitted('update:modelValue')).toBeTruthy();
});

test('duration must render seconds as formatted string', async () => {
	let rules = [{attribute: 'duration', operation: 'less', value: 600, invert: false}];
	let wrapper = instance({type: 'ruled', rules});

	let valueInput = wrapper.get('.rule-value');
	expect(valueInput.element.value).toBe('10:00');
});

test.each([
	'keydown.enter',
	'blur',
])('duration must accept seconds', async (triggerEvent) => {
	let rules = [{attribute: 'duration', operation: 'less', value: 600, invert: false}];
	let wrapper = instance({type: 'ruled', rules});

	let valueInput = wrapper.get('input.rule-value');
	expect(valueInput.element.value).toBe('10:00');

	valueInput.setValue('300');
	valueInput.trigger(triggerEvent);
	await wrapper.vm.$nextTick();
	expect(valueInput.element.value).toBe('05:00');
	expect(wrapper.emitted('update:modelValue')).toBeTruthy();
});
