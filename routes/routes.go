// tournament-system/routes/routes.go
package api

import (
	"github.com/Dosada05/tournament-system/handlers"
	"github.com/Dosada05/tournament-system/middleware"
	"github.com/Dosada05/tournament-system/models"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
)

func SetupRoutes(
	router *chi.Mux,
	authHandler *handlers.AuthHandler,
	userHandler *handlers.UserHandler,
	teamHandler *handlers.TeamHandler,
	tournamentHandler *handlers.TournamentHandler,
	sportHandler *handlers.SportHandler,
	inviteHandler *handlers.InviteHandler,
	participantHandler *handlers.ParticipantHandler,
	webSocketHandler *handlers.WebSocketHandler,
	formatHandler *handlers.FormatHandler,
	adminHandler *handlers.AdminUserHandler,
	dashboardHandler *handlers.DashboardHandler,
) {
	router.Use(chiMiddleware.Logger)
	router.Use(chiMiddleware.Recoverer)
	router.Use(chiMiddleware.RequestID)
	router.Use(chiMiddleware.RealIP)

	router.Route("/auth", func(r chi.Router) {
		r.Post("/signup", authHandler.Register)
		r.Post("/signin", authHandler.Login)

		r.Post("/forgot-password", authHandler.ForgotPassword)
		r.Post("/reset-password", authHandler.ResetPassword)
	})

	router.Route("/users", func(r chi.Router) {
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

			authRouter.Route("/{teamID}/invites", func(inviteRouter chi.Router) {
				inviteRouter.Post("/", inviteHandler.CreateOrRenewInviteHandler)
				inviteRouter.Get("/", inviteHandler.GetTeamInviteHandler)
				inviteRouter.Delete("/", inviteHandler.RevokeInviteHandler)
			})
		})
	})
	router.With(middleware.Authenticate).Post("/invites/join/{token}", inviteHandler.JoinTeamHandler)

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

	router.Route("/formats", func(r chi.Router) {
		r.Get("/", formatHandler.GetAllFormats)
		r.Get("/{formatID}", formatHandler.GetFormatByID)

		r.Group(func(adminRouter chi.Router) {
			adminRouter.Use(middleware.Authenticate)
			adminRouter.Use(middleware.Authorize(models.RoleAdmin))
			adminRouter.Post("/", formatHandler.CreateFormat)
			adminRouter.Put("/{formatID}", formatHandler.UpdateFormat)
			adminRouter.Delete("/{formatID}", formatHandler.DeleteFormat)
		})
	})

	router.Route("/tournaments", func(r chi.Router) {
		r.Get("/", tournamentHandler.ListHandler)
		r.Get("/{tournamentID}", tournamentHandler.GetByIDHandler)
		r.Get("/{tournamentID}/matches/solo", tournamentHandler.ListTournamentSoloMatchesHandler)
		r.Get("/{tournamentID}/matches/team", tournamentHandler.ListTournamentTeamMatchesHandler)
		r.Get("/{tournamentID}/participants", participantHandler.ListApplications)

		r.Get("/{tournamentID}/bracket", tournamentHandler.GetTournamentBracketHandler)

		r.Group(func(authRouter chi.Router) {
			authRouter.Use(middleware.Authenticate)
			authRouter.Post("/", tournamentHandler.CreateHandler)
			authRouter.Put("/{tournamentID}", tournamentHandler.UpdateDetailsHandler)
			authRouter.With(middleware.Authorize(models.RoleOrganizer, models.RoleAdmin)).Patch("/{tournamentID}/status", tournamentHandler.UpdateStatusHandler)
			authRouter.With(middleware.Authorize(models.RoleOrganizer, models.RoleAdmin)).Delete("/{tournamentID}", tournamentHandler.DeleteHandler)
			authRouter.With(middleware.Authorize(models.RoleOrganizer, models.RoleAdmin)).Post("/{tournamentID}/logo", tournamentHandler.UploadTournamentLogoHandler)

			authRouter.Post("/{tournamentID}/register/solo", participantHandler.RegisterSolo)
			authRouter.Post("/{tournamentID}/register/team", participantHandler.RegisterTeam)

			authRouter.With(middleware.Authorize(models.RoleOrganizer, models.RoleAdmin)).Patch("/{tournamentID}/matches/solo/{matchID}/result", tournamentHandler.UpdateSoloMatchResultHandler)
			authRouter.With(middleware.Authorize(models.RoleOrganizer, models.RoleAdmin)).Patch("/{tournamentID}/matches/team/{matchID}/result", tournamentHandler.UpdateTeamMatchResultHandler)
		})
	})

	router.Route("/participants/{participantID}", func(r chi.Router) {
		r.Use(middleware.Authenticate)
		r.Delete("/cancel", participantHandler.CancelRegistration)
		r.With(middleware.Authorize(models.RoleOrganizer, models.RoleAdmin)).Patch("/status", participantHandler.UpdateApplicationStatus)
	})

	router.With(middleware.Authenticate).Get("/ws/tournaments/{tournamentID}", webSocketHandler.ServeWs)

	router.Route("/admin", func(r chi.Router) {
		r.Use(middleware.Authenticate)
		r.Use(middleware.Authorize(models.RoleAdmin))

		r.Route("/users", func(r chi.Router) {
			r.Get("/", adminHandler.ListUsers)
			r.Delete("/{id}", adminHandler.DeleteUser)
			r.Get("/dashboard", dashboardHandler.Stats)
		})
	})

	router.Get("/confirm-email", authHandler.ConfirmEmail)
}
