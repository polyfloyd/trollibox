## v0.6.1 (2025-05-16)

### Fix

- Minor release for all dependency updates over the past period

## v0.6.0 (2024-02-18)

### Feat

- Bind Ctrl+K to the search prompt

### Refactor

- logrus -> log/slog

## v0.5.1 (2023-10-03)

### Fix

- **auto-queuer**: Shuffle instead of random pick the next track

## v0.5.0 (2022-07-31)

### Feat

- **autoqueuer**: Persist state between restarts
- Manage multiple auto queuer filters from the UI
- Ensure there is always a 'Default' auto queuer filter
- **autoqueuer**: Make the autoqueuer toggleable from the UI
- **queuer**: Set queuer filters through an API
- **player**: Automatically reconnect with visible countdown

### Refactor

- **events**: Close event listeners by context cancellation
- Remove unused PlayerLibrary functions
- **player**: Combine status calls into 1 function

## v0.4.1 (2022-07-04)

### Fix

- **rule-filter**: Fix string value not being saved

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
