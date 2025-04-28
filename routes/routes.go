package routes

import (
	"github.com/Dosada05/tournament-system/controllers"
	_ "github.com/Dosada05/tournament-system/docs"
	"github.com/Dosada05/tournament-system/middleware"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	httpSwagger "github.com/swaggo/http-swagger"
)

// InitRoutes принимает все контроллеры как параметры.
// Это позволяет внедрять зависимости и покрывать тестами роутинг.
func InitRoutes(
	userController *controllers.UserController,
	teamController *controllers.TeamController,
	sportController *controllers.SportController,
	tournamentController *controllers.TournamentController,
	participantController *controllers.ParticipantController,
) *chi.Mux {
	router := chi.NewRouter()

	// Middleware
	router.Use(chiMiddleware.Logger)
	router.Use(chiMiddleware.Recoverer)

	router.Use(cors.Handler(cors.Options{
		//AllowedOrigins:   []string{"http://192.168.56.1:5173"},
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Swagger UI
	router.Get("/swagger/*", httpSwagger.WrapHandler)

	// Пользовательские роуты
	router.Post("/users/signup", userController.SignUp)
	router.Post("/users/signin", userController.SignIn)
	router.Get("/users/{id}", userController.GetUser)
	router.Put("/users/{id}", userController.UpdateUser)
	router.Delete("/users/{id}", userController.DeleteUser)

	// Турниры
	router.Route("/tournaments", func(r chi.Router) {
		r.Get("/", tournamentController.GetAllTournaments)
		r.Get("/{id}", tournamentController.GetTournament)
		r.Group(func(r chi.Router) {
			r.Use(middleware.Authenticate)
			r.Use(middleware.Authorize("organizer"))
			r.Post("/", tournamentController.CreateTournament)
			r.Put("/{id}", tournamentController.UpdateTournament)
			r.Delete("/{id}", tournamentController.DeleteTournament)
		})
	})

	// Команды
	router.Route("/teams", func(r chi.Router) {
		r.Get("/", teamController.GetAllTeams)
		r.Get("/{id}", teamController.GetTeamByID)
		r.Group(func(r chi.Router) {
			r.Use(middleware.Authenticate)
			r.Use(middleware.Authorize("player"))
			r.Post("/", teamController.CreateTeam)
			r.Put("/{id}", teamController.UpdateTeam)
			r.Delete("/{id}", teamController.DeleteTeam)
		})
	})

	// Виды спорта
	router.Route("/sports", func(r chi.Router) {
		r.Get("/", sportController.GetAllSports)
		r.Get("/{id}", sportController.GetSport)
		r.Group(func(r chi.Router) {
			r.Use(middleware.Authenticate)
			// r.Use(middleware.Authorize("organizer"))
			r.Post("/", sportController.CreateSport)
			r.Put("/{id}", sportController.UpdateSport)
			r.Delete("/{id}", sportController.DeleteSport)
		})
	})

	// Участники турниров
	router.Route("/participants", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(middleware.Authenticate)
			r.Post("/user", participantController.RegisterUser)
			r.Post("/team", participantController.RegisterTeam)
			r.Put("/{id}/status", participantController.ChangeParticipantStatus)
			r.Delete("/{id}", participantController.Delete)
		})
		r.Get("/", participantController.ListByTournament)
		r.Get("/{id}", participantController.GetByID)
	})

	return router
}
