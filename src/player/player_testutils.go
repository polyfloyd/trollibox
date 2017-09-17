package player

import (
	"fmt"
	"time"
)

func TestPlaystateEvent(pl Player) error {
	l := pl.Events().Listen()
	defer pl.Events().Unlisten(l)

	if err := pl.SetState(PlayStatePlaying); err != nil {
		return err
	}
	if err := pl.SetState(PlayStateStopped); err != nil {
		return err
	}

	for {
		select {
		case msg := <-l:
			if msg == "playstate" {
				return nil
			}
		case <-time.After(time.Second):
			return fmt.Errorf("Event was not emitted")
		}
	}
}
