package main

import (
	"net/http"
)

func htBrowserPage() func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		params := GetBaseParamMap()

		err := RenderPage("view/browser.html", res, params)
		if err != nil {
			panic(err)
		}
	}
}

func htPlayerPage() func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		params := GetBaseParamMap()

		err := RenderPage("view/player.html", res, params)
		if err != nil {
			panic(err)
		}
	}
}
