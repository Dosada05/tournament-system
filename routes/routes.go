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

	router.Route("/tournaments", func(r chi.Router) {
		// Публичные маршруты для просмотра турниров
		r.Get("/", controllers.GetAllTournaments)
		r.Get("/{id}", controllers.GetTournament)

		// Защищенные маршруты только для организаторов
		r.Group(func(r chi.Router) {
			r.Use(middleware.Authenticate)
			r.Use(middleware.Authorize("organizer"))

			r.Post("/", controllers.CreateTournament)
			r.Put("/{id}", controllers.UpdateTournament)
			r.Delete("/{id}", controllers.DeleteTournament)
		})
	})

	return router
}
