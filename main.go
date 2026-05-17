package main

import "log"

func main() {
	cfg := LoadConfig()
	srv := NewMCPServer(cfg)
	if err := srv.Run(); err != nil {
		log.Fatal(err)
	}
}
