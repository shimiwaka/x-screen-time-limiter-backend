package main

import (
	"log"
	"net/http/cgi"
	"os"
	"path/filepath"
)

func main() {
	log.SetPrefix("[x-screen-time-limiter] ")
	exe, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}
	s := &server{dataFile: filepath.Join(filepath.Dir(exe), "data.json")}
	if err := cgi.Serve(s); err != nil {
		log.Fatal(err)
	}
}
