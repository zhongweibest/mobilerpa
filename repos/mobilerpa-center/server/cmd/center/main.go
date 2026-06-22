package main

import (
	"log"

	"github.com/mobilerpa/mobilerpa-center/server/internal/app"
)

func main() {
	application, err := app.New()
	if err != nil {
		log.Fatalf("bootstrap center: %v", err)
	}

	if err := application.Run(); err != nil {
		log.Fatalf("run center: %v", err)
	}
}
