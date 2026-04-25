package main

import "fmt"

func cmdVersion() int {
	fmt.Printf("lurpic version %s\n", version)
	fmt.Println("lurpicUI build tool")
	return 0
}
