package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func cmdClean(args []string) int {
	projectRoot, err := findProjectRoot()
	if err != nil {
		// If no project root found, just try to clean local build directory
		projectRoot, _ = os.Getwd()
	}

	buildDir := filepath.Join(projectRoot, "build")
	if _, err := os.Stat(buildDir); os.IsNotExist(err) {
		fmt.Println("Nothing to clean (no build directory)")
		return 0
	}

	if err := os.RemoveAll(buildDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error removing build directory: %v\n", err)
		return 1
	}

	fmt.Println("Build artifacts cleaned")
	return 0
}
