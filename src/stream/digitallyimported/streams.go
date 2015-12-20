package digitallyimported

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"

	"../../stream"
)

func Streams() ([]stream.Stream, error) {
	res, err := http.Get("https://www.di.fm/channels")
	if err != nil {
		return nil, fmt.Errorf("Unable to open di.fm: %v", err)
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("Unable to read di.fm: %v", err)
	}

	const dataStart = "di.app.start("
	jsonReader := bytes.NewReader(body[bytes.Index(body, []byte(dataStart))+len(dataStart):])

	data := struct {
		Channels []struct {
			Images struct {
				Default string `json:"default"`
			} `json:"images"`
			Key  string `json:"key"`
			Name string `json:"name"`
		} `json:"channels"`
	}{}

	if err := json.NewDecoder(jsonReader).Decode(&data); err != nil {
		return nil, fmt.Errorf("Unable to decode di.fm JSON: %v", err)
	}

	streams := make([]stream.Stream, len(data.Channels))
	artRegex := regexp.MustCompile("^(.+)\\{.+\\}$")
	for i, channel := range data.Channels {
		streams[i] = stream.Stream{
			Url:         fmt.Sprintf("http://pub1.di.fm/di_%s", channel.Key),
			StreamTitle: "DI " + channel.Name,
			ArtUrl: fmt.Sprintf(
				"https:%s?size=240x240",
				artRegex.FindStringSubmatch(channel.Images.Default)[1],
			),
		}
	}
	return streams, nil
}
