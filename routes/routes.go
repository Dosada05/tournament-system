package routes

import (
	"github.com/Dosada05/tournament-system/controllers"
	"github.com/Dosada05/tournament-system/middleware"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware" // Alias to avoid conflict
)

func InitRoutes() *chi.Mux {
	router := chi.NewRouter()

	router.Use(chiMiddleware.Logger)
	router.Use(chiMiddleware.Recoverer)

	router.Post("/users/signup", controllers.SignUp)
	router.Post("/users/signin", controllers.SignIn)

	// Protected Routes
	router.Group(func(r chi.Router) {
		r.Use(middleware.Authenticate)

		// User Routes
		r.Get("/users/{id}", controllers.GetUser)
		r.Put("/users/{id}", controllers.UpdateUser)
		r.Delete("/users/{id}", controllers.DeleteUser)

		// Team Routes
		r.With(middleware.Authorize("admin")).Post("/teams", controllers.CreateTeam)
		r.Get("/teams/{id}", controllers.GetTeam)
		r.With(middleware.Authorize("admin")).Put("/teams/{id}", controllers.UpdateTeam)
		r.With(middleware.Authorize("admin")).Delete("/teams/{id}", controllers.DeleteTeam)

		// Tournament Routes
		r.With(middleware.Authorize("admin")).Post("/tournaments", controllers.CreateTournament)
		r.Get("/tournaments/{id}", controllers.GetTournament)
		r.With(middleware.Authorize("admin")).Put("/tournaments/{id}", controllers.UpdateTournament)
		r.With(middleware.Authorize("admin")).Delete("/tournaments/{id}", controllers.DeleteTournament)

		// Participant Routes
		r.Post("/registrations", controllers.RegisterParticipant)
		r.Get("/registrations/{tournamentId}", controllers.GetParticipants)

		// Match Routes
		r.With(middleware.Authorize("admin")).Post("/matches", controllers.CreateMatch)
		r.Get("/matches/{id}", controllers.GetMatch)
		r.With(middleware.Authorize("admin")).Put("/matches/{id}", controllers.UpdateMatch)
		r.With(middleware.Authorize("admin")).Delete("/matches/{id}", controllers.DeleteMatch)
	})

	return router
}
