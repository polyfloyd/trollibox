// To be used as argument to Array#sort(). Compares strings without case
// sensitivity.
export function stringCompareCaseInsensitive(a, b) {
	a = a.toLowerCase();
	b = b.toLowerCase();
	return a > b ? 1
		: a < b ? -1
		: 0;
}
