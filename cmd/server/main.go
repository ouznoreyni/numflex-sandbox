package main

import (
	"log"

	"github.com/yas/numflex-sandbox/internal/config"
)

func main() {
	c, err := config.Load()
	if err != nil {
		log.Fatalf("configuration : %v", err)
	}
	log.Printf("numflex-sandbox — fidélité=%s expiration=%s port=%s",
		c.Fidelity, c.EtapeTimeout, c.Port)
}
