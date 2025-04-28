package main

import (
	"github.com/Dosada05/tournament-system/config"
	"github.com/Dosada05/tournament-system/controllers"
	"github.com/Dosada05/tournament-system/repositories"
	"github.com/Dosada05/tournament-system/routes"
	"github.com/Dosada05/tournament-system/services"
	"log"
	"net/http"
)

func main() {
	// 1. Подключения к базе данных
	db := config.InitDB()
	defer db.Close()

	// 2. Инициализация репозиториев (слой доступа к данным)
	userRepo := repositories.NewUserRepository(db)
	teamRepo := repositories.NewTeamRepository(db)
	sportRepo := repositories.NewSportRepository(db)
	tournamentRepo := repositories.NewTournamentRepository(db)
	participantRepo := repositories.NewParticipantRepository(db)

	// 3. Инициализация сервисов (бизнес-логика)
	userService := services.NewUserService(userRepo)
	teamService := services.NewTeamService(teamRepo, sportRepo)
	sportService := services.NewSportService(sportRepo)
	tournamentService := services.NewTournamentService(tournamentRepo)
	participantService := services.NewParticipantService(participantRepo, userRepo, teamRepo, tournamentRepo)

	// 4. Инициализация контроллеров (HTTP-слой)
	userController := controllers.NewUserController(userService)
	teamController := controllers.NewTeamController(teamService)
	sportController := controllers.NewSportController(sportService)
	tournamentController := controllers.NewTournamentController(tournamentService)
	participantController := controllers.NewParticipantController(participantService)

	// 5. Настройка маршрутов с внедрением зависимостей
	router := routes.InitRoutes(
		userController,
		teamController,
		sportController,
		tournamentController,
		participantController,
	)

	// 6. Запуск HTTP-сервера
	log.Println("Starting the server on :8080...")
	log.Fatal(http.ListenAndServe(":8080", router))
}
