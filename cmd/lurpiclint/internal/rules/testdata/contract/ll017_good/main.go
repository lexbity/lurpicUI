package main

import "codeburg.org/lexbit/lurpicui/app"

func main() {
	// OK: small config file, not a media resource.
	data, err := app.Asset("config.json")
	if err != nil {
		panic(err)
	}
	_ = data
}
