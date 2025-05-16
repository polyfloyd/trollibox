Trollibox
=========

The hackerspace friendly ~~MPD~~ music player web client.

Features:
* Control multiple music players from one webinterface
* Support for MPD
* Support for Logitech SlimServer and SqueezeBoxes
* Track art
* Listen to web radio stations
* Search-as-you-type for tracks with highlighting
* File browser
* Album browser
* Queue random tracks when the playlist is empty.
* Mobile device friendly
* Free Open Source Software (GPLv3)

## Installing
You can get the [latest pre-built binary](https://github.com/polyfloyd/trollibox/releases/latest)
from the Github Releases.

### Configuring
Copy the [example configuration](config.example.yaml) to config.yaml, its
default location. If you want to use a custom location for the config file, you
should inform Trollibox by using the `-conf` option. Like this:
```sh
trollibox -conf /etc/trollibox.yaml
```

Inside the configuration file, you will find some options you may need to change.

### Hacking
Clone the repository like you normally would. Make sure you have Go>1.18 installed along with
NodeJS/NPM.

You can start Trollibox with processes to automatically recompile the source like this:
```
$ make dev -j2
```


## For Users

### Hotkeys
* `space`: toggle playback
* `>`: next track
* `<`: previous track
* `+`: increase volume
* `-`: decrease volume
* `b`: seek backwards in the playing track
* `f`: seek forwards in the playing track
* `c`: clear the yet to be played part of the playlist
* `1`: focus library search
* `2`: focus album browser
* `3`: focus genre/artist browser
* `4`: focus file browser
* `5`: focus streams library
* `6`: focus auto queuer configuration
* `7`: expand player

### Queueing Tracks
Tracks may be queued from the browser page using one of the views. Click on a
track to append it to the queue.

An asterisk will be displayed next to tracks that have been queued by users.
This feature originated at the [Bitlair Hackerspace](https://bitlair.nl/) where
tracks should not be skipped when they are queued by users.

### The Queuer
If the queue runs out of tracks, Trollibox will pick a random track from the
library and play it. The selection bias for tracks can be configured by setting
one or more rules on the Queuer page.

Such rules consist of simple expressions that evaluate to a boolean value. A
track must pass all rules set for it to be eligible for playing.

The `equals` and `contains` operations are case insensitive.

The `matches` operation takes a regular expression in
[Go's regexp format](https://golang.org/pkg/regexp/syntax/).

### Streams
Trollibox has support for HTTP streams. You can create a custom collection
using the Streams interface.

### Searching
The search view allows you to search the whole library of the current player
for tracks whose artist, title or album attributes contain some keywords.

Searching is case insensitive.

A track must match all keywords in order to end up in the results.

The search string is split on each space, unless you escape it with a
backslash: `foo\ bar`.

You can annotate the keywords in your query to search other fields,
You can limit a keyword to a single attribute by annotating them like this:
```
photographer genre:trance
```
The string `photographer` will be applied to the default attributes while the
genre must match "trance".

Available attributes are:
* uri
* artist (default)
* title (default)
* genre
* album (default)
* albumartist
* albumtrack
* albumdisc

You can also use the `duration` attribute with relational operators to filter
on whether a track's length is less, greater than or equal to some reference
integer.
```
duration<120
duration>120
duration=120
```

## Q & A

#### What does the asterisk next to queued tracks indicate?
See [Queueing Tracks](#queueing-tracks).

#### Where is the button to update the library?
There isn't. Trollibox is only a browser/player. Managing the files of the
library is out of the scope of this project, which includes updating the
player's database.

#### How do I add stuff to the library?
Using whatever options the player you are using is giving you. Trollibox is
only a browser/player. You should manage your library in some other way.

#### I can't see the player on my phone.
The player is hidden on small screens to preserve space. The player is
accessible on a separate view for such devices.


## Screenshots
![Search for tracks](screenshots/1-search.png)

![Browse by album](screenshots/2-albums.png)

![Browse by genre and artist](screenshots/3-browse.png)

![Browse the filesystem](screenshots/4-files.png)

![Browse and add streams](screenshots/5-streams.png)

![The Queuer](screenshots/6-queuer.png)
