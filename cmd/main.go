package main

import (
	"github.com/Dosada05/tournament-system/config"
	"github.com/Dosada05/tournament-system/routes"
	"log"
	"net/http"
)

func main() {
	
	config.LoadConfig()

	router := routes.InitRoutes()

	log.Println("Starting the server on :8080...")
	log.Fatal(http.ListenAndServe(":8080", router))
}
