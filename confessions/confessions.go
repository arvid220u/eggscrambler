package main

import (
	"fmt"
	"github.com/arvid220u/6.824-project/anonbcast"
)

// TODO: extract the confessions app into its own package, and create a very simple
// 	main package that starts the confessions app.
func main() {
	println("welcome to the fully anonymous MIT confessions!")
	s := anonbcast.NewServer()
	c := anonbcast.NewClient(s)
	fmt.Printf("server: %v, client: %v\n", s, c)
}
