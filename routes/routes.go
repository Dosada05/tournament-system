package api

import (
	"github.com/Dosada05/tournament-system/handlers"
	"github.com/Dosada05/tournament-system/middleware"
	"github.com/Dosada05/tournament-system/models" // Нужно для middleware.Authorize

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	httpSwagger "github.com/swaggo/http-swagger"
)

func SetupRoutes(
	router *chi.Mux,
	authHandler *handlers.AuthHandler,
	userHandler *handlers.UserHandler,
	teamHandler *handlers.TeamHandler,
	tournamentHandler *handlers.TournamentHandler,
	sportHandler *handlers.SportHandler,
	inviteHandler *handlers.InviteHandler,
) {
	router.Use(chiMiddleware.Logger)
	router.Use(chiMiddleware.Recoverer)
	router.Use(chiMiddleware.RequestID)
	router.Use(chiMiddleware.RealIP)

	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	router.Get("/swagger/*", httpSwagger.WrapHandler)

	router.Route("/users", func(r chi.Router) {
		r.Post("/signup", authHandler.Register)
		r.Post("/signin", authHandler.Login)
		r.Get("/{id}", userHandler.GetUserByID)

		r.Group(func(authRouter chi.Router) {
			authRouter.Use(middleware.Authenticate)
			authRouter.Put("/{id}", userHandler.UpdateUserByID)
			authRouter.Post("/{id}/avatar", userHandler.UploadUserLogo)
		})
	})

	router.Route("/teams", func(r chi.Router) {
		r.Get("/{teamID}", teamHandler.GetTeamByID)
		r.Get("/{teamID}/members", teamHandler.ListTeamMembers)

		r.Group(func(authRouter chi.Router) {
			authRouter.Use(middleware.Authenticate)
			authRouter.Post("/", teamHandler.CreateTeam)
			authRouter.Put("/{teamID}", teamHandler.UpdateTeamDetails)
			authRouter.Delete("/{teamID}", teamHandler.DeleteTeam)
			authRouter.Delete("/{teamID}/members/{userID}", teamHandler.RemoveMember)
			authRouter.Post("/{teamID}/logo", teamHandler.UploadTeamLogo)
		})
	})

	router.Route("/tournaments", func(r chi.Router) {
		r.Get("/", tournamentHandler.ListHandler)
		r.Get("/{tournamentID}", tournamentHandler.GetByIDHandler)

		r.Group(func(authRouter chi.Router) {
			authRouter.Use(middleware.Authenticate)
			authRouter.Post("/", tournamentHandler.CreateHandler)
			authRouter.Put("/{tournamentID}", tournamentHandler.UpdateDetailsHandler)
			authRouter.Patch("/{tournamentID}/status", tournamentHandler.UpdateStatusHandler)
			authRouter.Delete("/{tournamentID}", tournamentHandler.DeleteHandler)
			authRouter.Post("/{tournamentID}/logo", tournamentHandler.UploadTournamentLogoHandler)
		})
	})

	router.Route("/sports", func(r chi.Router) {
		r.Get("/", sportHandler.GetAllSports)
		r.Get("/{sportID}", sportHandler.GetSportByID)

		r.Group(func(adminRouter chi.Router) {
			adminRouter.Use(middleware.Authenticate)
			adminRouter.Use(middleware.Authorize(models.RoleAdmin))

			adminRouter.Post("/", sportHandler.CreateSport)
			adminRouter.Put("/{sportID}", sportHandler.UpdateSport)
			adminRouter.Delete("/{sportID}", sportHandler.DeleteSport)
			adminRouter.Post("/{sportID}/logo", sportHandler.UploadSportLogoHandler)
		})
	})

	router.Group(func(authRouter chi.Router) {
		authRouter.Use(middleware.Authenticate)
		authRouter.Post("/invites/{token}", inviteHandler.JoinTeamHandler)
	})

	router.Route("/teams/{teamID}/invites", func(r chi.Router) {
		r.Use(middleware.Authenticate)
		r.Post("/", inviteHandler.CreateOrRenewInviteHandler)
		r.Get("/", inviteHandler.GetTeamInviteHandler)
		r.Delete("/", inviteHandler.RevokeInviteHandler)
	})
}
