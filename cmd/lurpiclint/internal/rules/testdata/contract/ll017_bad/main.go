package main

import "codeburg.org/lexbit/lurpicui/app"

func main() {
	// LL017: media file via app.Asset; use Manager.LoadImage instead.
	data, err := app.Asset("icon.png")
	if err != nil {
		panic(err)
	}
	_ = data
}
