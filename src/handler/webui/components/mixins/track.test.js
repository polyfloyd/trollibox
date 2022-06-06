import TrackMixin from './track.js';



let durationToStringCases = [
  [0, '00:00'],
  [4, '00:04'],
  [10, '00:10'],
  [16, '00:16'],
  [62, '01:02'],
  [240, '04:00'],
  [7260, '02:01:00'],
];
test.each(durationToStringCases)('durationToString', (seconds, string) => {
  let result = TrackMixin.methods.durationToString(seconds);
  expect(result).toBe(string);
});

