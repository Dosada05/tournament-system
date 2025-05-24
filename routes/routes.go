// tournament-system/routes/routes.go
package api

import (
	"github.com/Dosada05/tournament-system/handlers"
	"github.com/Dosada05/tournament-system/middleware"
	"github.com/Dosada05/tournament-system/models"

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
	participantHandler *handlers.ParticipantHandler,
	webSocketHandler *handlers.WebSocketHandler,
	formatHandler *handlers.FormatHandler,
) {
	router.Use(chiMiddleware.Logger)
	router.Use(chiMiddleware.Recoverer)
	router.Use(chiMiddleware.RequestID)
	router.Use(chiMiddleware.RealIP)

	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"}, // Настрой для продакшена!
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	router.Get("/swagger/*", httpSwagger.WrapHandler)

	// --- Маршруты Auth ---
	router.Route("/auth", func(r chi.Router) { // Изменил с /users на /auth для ясности
		r.Post("/signup", authHandler.Register)
		r.Post("/signin", authHandler.Login)
	})

	// --- Маршруты Users ---
	router.Route("/users", func(r chi.Router) {
		r.Get("/{id}", userHandler.GetUserByID) // Публичный профиль

		r.Group(func(authRouter chi.Router) {
			authRouter.Use(middleware.Authenticate)
			authRouter.Put("/{id}", userHandler.UpdateUserByID)
			authRouter.Post("/{id}/avatar", userHandler.UploadUserLogo)
		})
	})

	// --- Маршруты Teams ---
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

			// Маршруты для инвайтов в команду
			authRouter.Route("/{teamID}/invites", func(inviteRouter chi.Router) {
				inviteRouter.Post("/", inviteHandler.CreateOrRenewInviteHandler)
				inviteRouter.Get("/", inviteHandler.GetTeamInviteHandler)
				inviteRouter.Delete("/", inviteHandler.RevokeInviteHandler)
			})
		})
	})
	// Маршрут для присоединения к команде по токену (аутентифицированный пользователь)
	router.With(middleware.Authenticate).Post("/invites/join/{token}", inviteHandler.JoinTeamHandler)

	// --- Маршруты Sports ---
	router.Route("/sports", func(r chi.Router) {
		r.Get("/", sportHandler.GetAllSports)
		r.Get("/{sportID}", sportHandler.GetSportByID)

		r.Group(func(adminRouter chi.Router) {
			adminRouter.Use(middleware.Authenticate)
			adminRouter.Use(middleware.Authorize(models.RoleAdmin)) // Только админ может управлять видами спорта
			adminRouter.Post("/", sportHandler.CreateSport)
			adminRouter.Put("/{sportID}", sportHandler.UpdateSport)
			adminRouter.Delete("/{sportID}", sportHandler.DeleteSport)
			adminRouter.Post("/{sportID}/logo", sportHandler.UploadSportLogoHandler)
		})
	})

	router.Route("/formats", func(r chi.Router) {
		// Публичные маршруты для форматов (если нужны)
		r.Get("/", formatHandler.GetAllFormats)           // Список всех форматов
		r.Get("/{formatID}", formatHandler.GetFormatByID) // Получение формата по ID

		// Маршруты, требующие аутентификации и прав администратора
		r.Group(func(adminRouter chi.Router) {
			adminRouter.Use(middleware.Authenticate)
			adminRouter.Use(middleware.Authorize(models.RoleAdmin))
			adminRouter.Post("/", formatHandler.CreateFormat)
			adminRouter.Put("/{formatID}", formatHandler.UpdateFormat)
			adminRouter.Delete("/{formatID}", formatHandler.DeleteFormat)
		})
	})

	// --- Маршруты Tournaments ---
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

	// --- Маршруты Participants (управление заявками) ---
	router.Route("/participants/{participantID}", func(r chi.Router) {
		r.Use(middleware.Authenticate)
		// Отмена своей регистрации (пользователь/капитан)
		r.Delete("/cancel", participantHandler.CancelRegistration)
		// Управление заявками (для организатора турнира - можно добавить Authorize)
		r.With(middleware.Authorize(models.RoleOrganizer, models.RoleAdmin)).Patch("/status", participantHandler.UpdateApplicationStatus)
	})

	router.With(middleware.Authenticate).Get("/ws/tournaments/{tournamentID}", webSocketHandler.ServeWs)

}
