package main

import (
	"log"
	"supervisor-demo/internal/component"
)

func main() {
	if err := component.Mock(); err != nil {
		log.Fatal(err)
	}
}
