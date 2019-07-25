package main

import (
	"fmt"
	"net/http"
)

func main() {
	// not used - make the linter happy
	EntryPoint(nil, nil)
}

// EntryPoint is the entry point for this Fission function
func EntryPoint(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Hello from Fission")
}
