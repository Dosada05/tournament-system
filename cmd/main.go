package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/Dosada05/tournament-system/db"
	api "github.com/Dosada05/tournament-system/routes"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Dosada05/tournament-system/config"
	_ "github.com/Dosada05/tournament-system/db"
	"github.com/Dosada05/tournament-system/handlers"
	"github.com/Dosada05/tournament-system/repositories"
	"github.com/Dosada05/tournament-system/services"

	"github.com/go-chi/chi/v5"
	_ "github.com/lib/pq"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load configuration", slog.Any("error", err))
		os.Exit(1)
	}

	logger.Info("configuration loaded", slog.Int("port", cfg.ServerPort))

	db, err := db.Connect(cfg.DatabaseURL, 5*time.Second)
	if err != nil {
		logger.Error("failed to connect to database", slog.Any("error", err))
		os.Exit(1)
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.Error("failed to close database connection", slog.Any("error", err))
		} else {
			logger.Info("database connection closed")
		}
	}()

	logger.Info("database connection established")

	userRepo := repositories.NewPostgresUserRepository(db)
	teamRepo := repositories.NewPostgresTeamRepository(db)
	sportRepo := repositories.NewPostgresSportRepository(db)
	//formatRepo := repositories.NewPostgresFormatRepository(db)
	//tournamentRepo := repositories.NewPostgresTournamentRepository(db)
	//participantRepo := repositories.NewPostgresParticipantRepository(db)
	inviteRepo := repositories.NewPostgresInviteRepository(db)

	authService := services.NewAuthService(userRepo)
	userService := services.NewUserService(userRepo)
	sportService := services.NewSportService(sportRepo)
	//formatService := services.NewFormatService(formatRepo)
	teamService := services.NewTeamService(teamRepo, userRepo, sportRepo)
	inviteService := services.NewInviteService(inviteRepo, teamRepo, userRepo)
	//participantService := services.NewParticipantService(participantRepo, tournamentRepo, userRepo, teamRepo)
	//tournamentService := services.NewTournamentService(tournamentRepo, sportRepo, formatRepo, userRepo, participantService)

	authHandler := handlers.NewAuthHandler(authService, cfg.JWTSecretKey)
	userHandler := handlers.NewUserHandler(userService)
	teamHandler := handlers.NewTeamHandler(teamService, userService)
	sportHandler := handlers.NewSportHandler(sportService)
	// formatHandler := handlers.NewFormatHandler(formatService)
	// tournamentHandler := handlers.NewTournamentHandler(tournamentService)
	// participantHandler := handlers.NewParticipantHandler(participantService, tournamentService)
	inviteHandler := handlers.NewInviteHandler(inviteService)
		
	router := chi.NewRouter()

	api.SetupRoutes(
		router,
		authHandler,
		userHandler,
		teamHandler,
		sportHandler,
		inviteHandler,
		// tournamentHandler,
		// participantHandler,
	)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.ServerPort),
		Handler:      router,
		ReadTimeout:  5 * time.Second,
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
			logger.Info("server stopped")
		}
	case sig := <-quit:
		logger.Info("shutdown signal received", slog.String("signal", sig.String()))

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		logger.Info("shutting down server", slog.Duration("timeout", 15*time.Second))
		if err := server.Shutdown(ctx); err != nil {
			logger.Error("graceful shutdown failed", slog.Any("error", err))
			if closeErr := server.Close(); closeErr != nil {
				logger.Error("failed to force close server", slog.Any("error", closeErr))
			}
			os.Exit(1)
		} else {
			logger.Info("server shutdown complete")
		}
	}

	logger.Info("server exited")
}
