package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/go-chi/cors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Dosada05/tournament-system/brackets"
	"github.com/Dosada05/tournament-system/config"
	"github.com/Dosada05/tournament-system/db"
	"github.com/Dosada05/tournament-system/handlers"
	"github.com/Dosada05/tournament-system/repositories"
	api "github.com/Dosada05/tournament-system/routes"
	"github.com/Dosada05/tournament-system/services"
	"github.com/Dosada05/tournament-system/storage"
	"github.com/go-chi/chi/v5"
	_ "github.com/lib/pq"
)

const schedulerInterval = 30 * time.Second

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load configuration", slog.Any("error", err))
		os.Exit(1)
	}
	logger.Info("configuration loaded", slog.Int("port", cfg.ServerPort))

	dbConn, err := db.Connect(cfg.DatabaseURL, 5*time.Second)
	if err != nil {
		logger.Error("failed to connect to database", slog.Any("error", err))
		os.Exit(1)
	}
	defer func() {
		if err := dbConn.Close(); err != nil {
			logger.Error("failed to close database connection", slog.Any("error", err))
		} else {
			logger.Info("database connection closed")
		}
	}()
	logger.Info("database connection established")

	cloudflareUploader, err := storage.NewCloudflareR2Uploader(storage.CloudflareR2UploaderConfig{
		AccountID:       cfg.R2AccountID,
		AccessKeyID:     cfg.R2AccessKeyID,
		SecretAccessKey: cfg.R2SecretAccessKey,
		BucketName:      cfg.R2BucketName,
		PublicBaseURL:   cfg.R2PublicBaseURL,
	})
	if err != nil {
		logger.Error("failed to initialize Cloudflare R2 uploader", slog.Any("error", err))
		os.Exit(1)
	}
	logger.Info("Cloudflare R2 uploader initialized")

	wsHub := brackets.NewHub()
	go wsHub.Run()
	logger.Info("WebSocket Hub started")

	userRepo := repositories.NewPostgresUserRepository(dbConn)
	teamRepo := repositories.NewPostgresTeamRepository(dbConn)
	sportRepo := repositories.NewPostgresSportRepository(dbConn)
	formatRepo := repositories.NewPostgresFormatRepository(dbConn)
	tournamentRepo := repositories.NewPostgresTournamentRepository(dbConn)
	inviteRepo := repositories.NewPostgresInviteRepository(dbConn)
	participantRepo := repositories.NewPostgresParticipantRepository(dbConn)
	soloMatchRepo := repositories.NewPostgresSoloMatchRepository(dbConn)
	teamMatchRepo := repositories.NewPostgresTeamMatchRepository(dbConn)
	standingRepo := repositories.NewPostgresTournamentStandingRepository(dbConn)
	logger.Info("Repositories initialized")

	authService := services.NewAuthService(userRepo)
	userService := services.NewUserService(userRepo, cloudflareUploader)
	sportService := services.NewSportService(sportRepo, userRepo, cloudflareUploader)
	formatService := services.NewFormatService(formatRepo)
	teamService := services.NewTeamService(teamRepo, userRepo, sportRepo, cloudflareUploader)
	inviteService := services.NewInviteService(inviteRepo, teamRepo, userRepo)
	adminService := services.NewAdminUserService(userRepo)

	dashboardService := services.NewDashboardService(userRepo, tournamentRepo, soloMatchRepo, teamMatchRepo)

	bracketService := services.NewBracketService(
		formatRepo,
		participantRepo,
		soloMatchRepo,
		teamMatchRepo,
		standingRepo,
		logger,
	)

	matchService := services.NewMatchService(
		dbConn,
		soloMatchRepo,
		teamMatchRepo,
		tournamentRepo,
		participantRepo,
		formatRepo,
		standingRepo,
		wsHub,
		logger,
	)

	tournamentService := services.NewTournamentService(
		dbConn,
		tournamentRepo,
		sportRepo,
		formatRepo,
		userRepo,
		participantRepo,
		soloMatchRepo,
		teamMatchRepo,
		standingRepo,
		bracketService,
		matchService,
		cloudflareUploader,
		wsHub,
		logger,
	)
	participantService := services.NewParticipantService(
		participantRepo,
		tournamentRepo,
		userRepo,
		teamRepo,
		formatRepo,
		cloudflareUploader,
	)
	logger.Info("Services initialized")

	go func() {
		ticker := time.NewTicker(schedulerInterval)
		defer ticker.Stop()
		logger.Info("Tournament status update scheduler started", slog.Duration("interval", schedulerInterval))
		if err := tournamentService.AutoUpdateTournamentStatusesByDates(context.Background()); err != nil {
			logger.Error("Scheduler: initial run failed", slog.Any("error", err))
		}

		for {
			select {
			case <-ticker.C:
				logger.Info("Scheduler: triggering automatic tournament status update.")
				if err := tournamentService.AutoUpdateTournamentStatusesByDates(context.Background()); err != nil {
					logger.Error("Scheduler: periodic run failed", slog.Any("error", err))
				}
			}
		}
	}()

	authHandler := handlers.NewAuthHandler(authService, cfg.JWTSecretKey)
	userHandler := handlers.NewUserHandler(userService)
	teamHandler := handlers.NewTeamHandler(teamService, userService)
	sportHandler := handlers.NewSportHandler(sportService)
	formatHandler := handlers.NewFormatHandler(formatService)
	tournamentHandler := handlers.NewTournamentHandler(tournamentService, matchService)
	inviteHandler := handlers.NewInviteHandler(inviteService)
	participantHandler := handlers.NewParticipantHandler(participantService)
	webSocketHandler := handlers.NewWebSocketHandler(wsHub)
	adminHandler := handlers.NewAdminUserHandler(adminService)
	dashboardHandler := handlers.NewDashboardHandler(dashboardService)
	logger.Info("HTTP handlers initialized")

	router := chi.NewRouter()
	api.SetupRoutes(
		router,
		authHandler,
		userHandler,
		teamHandler,
		tournamentHandler,
		sportHandler,
		inviteHandler,
		participantHandler,
		webSocketHandler,
		formatHandler,
		adminHandler,
		dashboardHandler,
	)
	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   []string{"heartbit.live"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: true,
	}).Handler(router)

	logger.Info("Routes configured")

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.ServerPort),
		Handler:      corsHandler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
		ErrorLog:     slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("starting server", slog.String("address", server.Addr))
		serverErrors <- server.ListenAndServe()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", slog.Any("error", err))
			os.Exit(1)
		} else {
			logger.Info("server stopped gracefully")
		}
	case sig := <-quit:
		logger.Info("shutdown signal received", slog.String("signal", sig.String()))
		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancelShutdown()

		logger.Info("shutting down server", slog.Duration("timeout", 15*time.Second))
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("graceful shutdown failed", slog.Any("error", err))
			if closeErr := server.Close(); closeErr != nil {
				logger.Error("failed to force close server", slog.Any("error", closeErr))
			}
			os.Exit(1)
		} else {
			logger.Info("server shutdown complete")
		}
	}
	logger.Info("application exited")
}
