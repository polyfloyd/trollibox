## v0.4.0 (2022-07-03)

### Refactor

- **filters/db**: Reimplement events
- Make browser tab logic reusable
- **rule-filter**: Extract queuer editor into its own component
- Move stream metadata loading to the jukebox package
- **mpd**: Simplify some withMpd invocations
- Use a sentinel error for PlayerByName

### Fix

- Ensure eventsource connections terminate
- **search**: Fix adding a track without album

### Feat

- **rule-filter**: Remove hidden number formats for duration input
- Default to the stream URL for unset stream titles

### Perf

- Enable pprof on debug builds

## v0.3.0 (2022-06-13)

### Fix

- **mpd**: Load library with true recursion

### Feat

- Jump to an album from the search results list
- **filter/ruled**: Make Equals and Contains operations case insensitive

### Refactor

- **filter/ruled**: Make test output easier to read
- **filter/ruled**: Introduce Op type

## v0.2.2 (2022-06-02)

### Fix

- Clear playlist button

## v0.2.1 (2022-05-16)

## v0.2.0 (2022-05-15)

## v0.1.0 (2018-03-26)
