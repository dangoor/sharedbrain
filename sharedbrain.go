package main

import (
	"flag"
	"log"
	"sharedbrain/backlinker"
)

const VERSION = "1.1"

type builder struct {
	dist bool
}

func main() {
	content := flag.String("content", "", "Source directory")
	dest := flag.String("dest", "", "Destination directory")
	version := flag.Bool("v", false, "Prints version")
	flag.Parse()

	log.Printf("sharedbrain %s\n", VERSION)
	if *version {
		log.Print("(just printing version, at your request)\n")
		return
	}

	if *dest == "" || *content == "" {
		log.Fatal("Either dest or content have not been set. Cannot proceed.\n")
	}
	err := backlinker.ProcessBackLinks(*content, *dest)
	if err != nil {
		log.Fatalf("Error when processing: %v\n", err)
	}
	log.Print("Generation complete!\n")
}
