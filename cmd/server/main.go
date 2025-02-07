package main

import (
	"log"

	"github.com/i0li/proglog/internal/server"
)

func main() {
	srv := server.NewHTTPServer(":8000")
	log.Fatal(srv.ListenAndServe())
}
