package api

import (
	"net/http"

	"github.com/Dosada05/tournament-system/handlers"
	"github.com/Dosada05/tournament-system/middleware" // Используем твой путь к middleware

	_ "github.com/Dosada05/tournament-system/docs"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	httpSwagger "github.com/swaggo/http-swagger"
	// _ "github.com/Dosada05/tournament-system/docs" // Раскомментируй, если используешь Swagger docs
)

func SetupRoutes(
	router *chi.Mux,
	authHandler *handlers.AuthHandler,
	// Добавь здесь другие хендлеры, когда они будут готовы:
	// userHandler *handlers.UserHandler,
	// teamHandler *handlers.TeamHandler,
	// tournamentHandler *handlers.TournamentHandler,
	// participantHandler *handlers.ParticipantHandler,
	// sportHandler *handlers.SportHandler,
	// inviteHandler *handlers.InviteHandler, // Для приглашений
) {

	router.Use(chiMiddleware.Logger)
	router.Use(chiMiddleware.Recoverer)
	router.Use(chiMiddleware.RequestID) // Полезно для трассировки
	router.Use(chiMiddleware.RealIP)    // Получение реального IP клиента

	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"}, // ВНИМАНИЕ: Замени на домены фронтенда в продакшене
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	router.Get("/swagger/*", httpSwagger.WrapHandler)

	// --- Публичные маршруты для аутентификации ---
	// Используем префикс /users, как в твоем примере
	router.Post("/users/signup", authHandler.Register) // Был userController.SignUp
	router.Post("/users/signin", authHandler.Login)    // Был userController.SignIn

	// --- Маршруты для пользователей (требуют UserHandler) ---
	// router.Get("/users/{id}", userHandler.GetUser) // Был userController.GetUser
	// router.Put("/users/{id}", userHandler.UpdateUser) // Был userController.UpdateUser
	// router.Delete("/users/{id}", userHandler.DeleteUser) // Был userController.DeleteUser
	// Рекомендуется добавить маршрут для получения/обновления своего профиля:
	// router.Route("/users/me", func(r chi.Router) {
	//  r.Use(middleware.Authenticate)
	//  r.Get("/", userHandler.GetMe)
	//  r.Put("/", userHandler.UpdateMe)
	// })

	// --- Маршруты для турниров (требуют TournamentHandler и ParticipantHandler) ---
	router.Route("/tournaments", func(r chi.Router) {
		// r.Get("/", tournamentHandler.ListTournaments)    // Был tournamentController.GetAllTournaments (публичный?)
		// r.Get("/{id}", tournamentHandler.GetTournament) // Был tournamentController.GetTournament (публичный?)

		// Группа для защищенных действий с турнирами
		r.Group(func(secureRouter chi.Router) {
			secureRouter.Use(middleware.Authenticate) // Общая аутентификация

			// Действия организатора
			secureRouter.Group(func(organizerRouter chi.Router) {
				organizerRouter.Use(middleware.Authorize("organizer"))
				// organizerRouter.Post("/", tournamentHandler.CreateTournament) // Был tournamentController.CreateTournament
				// organizerRouter.Put("/{id}", tournamentHandler.UpdateTournament) // Был tournamentController.UpdateTournament
				// organizerRouter.Delete("/{id}", tournamentHandler.DeleteTournament) // Был tournamentController.DeleteTournament
			})

			// Регистрация на турнир (доступно аутентифицированным пользователям)
			// Нужны participantHandler
			// secureRouter.Post("/{tournamentID}/participants/solo", participantHandler.RegisterSolo) // Адаптировано из participantController.RegisterUser
			// secureRouter.Post("/{tournamentID}/participants/team", participantHandler.RegisterTeam) // Адаптировано из participantController.RegisterTeam
		})
	})

	// --- Маршруты для команд (требуют TeamHandler и InviteHandler) ---
	router.Route("/teams", func(r chi.Router) {
		// r.Get("/", teamHandler.ListTeams)    // Был teamController.GetAllTeams (публичный?)
		// r.Get("/{id}", teamHandler.GetTeamByID) // Был teamController.GetTeamByID (публичный?)

		// Группа для защищенных действий с командами
		r.Group(func(secureRouter chi.Router) {
			secureRouter.Use(middleware.Authenticate)

			// Обычно создание и управление командой доступно роли 'player'
			secureRouter.Group(func(playerRouter chi.Router) {
				// playerRouter.Use(middleware.Authorize("player")) // Если нужно ограничить только игроками
				// playerRouter.Post("/", teamHandler.CreateTeam) // Был teamController.CreateTeam
				// playerRouter.Put("/{id}", teamHandler.UpdateTeam) // Был teamController.UpdateTeam (обычно капитаном)
				// playerRouter.Delete("/{id}", teamHandler.DeleteTeam) // Был teamController.DeleteTeam (обычно капитаном)

				// Управление приглашениями (капитаном)
				// playerRouter.Post("/{teamID}/invites", inviteHandler.CreateInvite)
				// playerRouter.Get("/{teamID}/invites", inviteHandler.ListTeamInvites)
				// playerRouter.Delete("/{teamID}/invites/{inviteID}", inviteHandler.DeleteInvite)
			})
		})
	})

	// --- Маршруты для видов спорта (требуют SportHandler) ---
	router.Route("/sports", func(r chi.Router) {
		// r.Get("/", sportHandler.ListSports) // Был sportController.GetAllSports (публичный?)
		// r.Get("/{id}", sportHandler.GetSport) // Был sportController.GetSport (публичный?)

		// Группа для защищенных действий (создание/редактирование спорта)
		r.Group(func(secureRouter chi.Router) {
			secureRouter.Use(middleware.Authenticate)
			// secureRouter.Use(middleware.Authorize("admin", "organizer")) // Пример: доступ админам или организаторам
			// secureRouter.Post("/", sportHandler.CreateSport) // Был sportController.CreateSport
			// secureRouter.Put("/{id}", sportHandler.UpdateSport) // Был sportController.UpdateSport
			// secureRouter.Delete("/{id}", sportHandler.DeleteSport) // Был sportController.DeleteSport
		})
	})

	// --- Маршруты для участников (требуют ParticipantHandler) ---
	// Некоторые маршруты уже добавлены под /tournaments/{tournamentID}/participants
	// Здесь могут быть маршруты для получения информации об участии или отмены
	router.Route("/participants", func(r chi.Router) {
		// r.Get("/", participantHandler.ListUserRegistrations) // Получить все регистрации текущего пользователя?
		// r.Get("/{id}", participantHandler.GetByID) // Был participantController.GetByID

		r.Group(func(secureRouter chi.Router) {
			secureRouter.Use(middleware.Authenticate)
			// secureRouter.Put("/{id}/status", participantHandler.ChangeParticipantStatus) // Был participantController.ChangeParticipantStatus (доступ организатора?)
			// secureRouter.Delete("/{id}", participantHandler.CancelRegistration) // Был participantController.Delete (отмена своей регистрации)
		})
	})

	// --- Маршруты для приглашений (требуют InviteHandler) ---
	router.Group(func(r chi.Router) {
		r.Use(middleware.Authenticate)
		// r.Post("/invites/{token}/accept", inviteHandler.AcceptInvite)
	})

	// --- Health Check ---
	router.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK\n"))
	})
}
