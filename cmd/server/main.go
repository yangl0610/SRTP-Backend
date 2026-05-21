package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/QSCTech/SRTP-Backend/internal/api"
	"github.com/QSCTech/SRTP-Backend/internal/config"
	"github.com/QSCTech/SRTP-Backend/internal/database"
	applog "github.com/QSCTech/SRTP-Backend/internal/logger"
	"github.com/QSCTech/SRTP-Backend/internal/repository"
	"github.com/QSCTech/SRTP-Backend/internal/service"
	"github.com/QSCTech/SRTP-Backend/internal/zjulogin"
	"github.com/QSCTech/SRTP-Backend/models"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	log, err := applog.New(cfg.AppEnv, cfg.LogLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "init logger: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = log.Sync() }()

	log = log.With(zap.String("service", "srtp-backend"), zap.String("env", cfg.AppEnv))

	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	gormDB, err := database.NewPostgres(cfg, log)
	if err != nil {
		log.Fatal("initialize database", zap.Error(err))
	}

	if err := gormDB.AutoMigrate(
		&models.User{},
		&models.Room{},
		&models.RoomMember{},
		&models.JoinRequest{},
		&models.RoomReservation{},
		&models.ReservationAttemptLog{},
		&models.UserProfileAudit{},
		&models.Notification{},
	); err != nil {
		log.Fatal("auto migrate models", zap.Error(err))
	}

	if err := preparePublicIDColumns(gormDB); err != nil {
		log.Fatal("prepare public_id columns", zap.Error(err))
	}

	if err := backfillPublicIDs(gormDB); err != nil {
		log.Fatal("backfill public ids", zap.Error(err))
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		log.Fatal("get sql db", zap.Error(err))
	}
	defer func() { _ = sqlDB.Close() }()

	userRepository := repository.NewUserRepository(gormDB)
	userService := service.NewUserService(userRepository)
	roomRepository := repository.NewRoomRepository(gormDB)
	roomService := service.NewRoomService(roomRepository, userService)
	reservationRepository := repository.NewReservationRepository(gormDB)

	// Initialize ZJUZJL login for TYYS reservation system.
	auth, err := zjulogin.NewFromEnv()
	if err != nil {
		log.Fatal("initialize zjulogin", zap.Error(err))
	}
	tyys, err := auth.TYYS()
	if err != nil {
		log.Fatal("initialize TYYS client", zap.Error(err))
	}
	reservationService := service.NewReservationService(roomRepository, reservationRepository, tyys)
	engine := api.NewRouter(log, sqlDB, userService, roomService, reservationService)

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:           engine,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		log.Info("http server started", zap.Int("port", cfg.HTTPPort))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-signalCtx.Done():
		log.Info("shutdown signal received")
	case err := <-errCh:
		if err != nil {
			log.Fatal("http server stopped", zap.Error(err))
		}
		return
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown failed", zap.Error(err))
		if closeErr := server.Close(); closeErr != nil {
			log.Error("force close failed", zap.Error(closeErr))
		}
		os.Exit(1)
	}

	log.Info("server stopped")
}

func preparePublicIDColumns(db *gorm.DB) error {
	for _, table := range []string{
		"users",
		"rooms",
		"join_requests",
		"room_reservations",
		"notifications",
	} {
		if err := db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN IF NOT EXISTS public_id uuid", table)).Error; err != nil {
			return fmt.Errorf("prepare %s.public_id: %w", table, err)
		}
	}
	return nil
}

func backfillPublicIDs(gormDB *gorm.DB) error {
	if err := backfillUserPublicIDs(gormDB); err != nil {
		return err
	}
	if err := backfillRoomPublicIDs(gormDB); err != nil {
		return err
	}
	if err := backfillJoinRequestPublicIDs(gormDB); err != nil {
		return err
	}
	if err := backfillRoomReservationPublicIDs(gormDB); err != nil {
		return err
	}
	if err := backfillNotificationPublicIDs(gormDB); err != nil {
		return err
	}
	return nil
}

func backfillUserPublicIDs(db *gorm.DB) error {
	var items []models.User
	if err := db.Where("public_id IS NULL").Find(&items).Error; err != nil {
		return err
	}
	for _, item := range items {
		if err := db.Model(&models.User{}).Where("id = ?", item.ID).Update("public_id", models.NewPublicID()).Error; err != nil {
			return err
		}
	}
	return nil
}

func backfillRoomPublicIDs(db *gorm.DB) error {
	var items []models.Room
	if err := db.Where("public_id IS NULL").Find(&items).Error; err != nil {
		return err
	}
	for _, item := range items {
		if err := db.Model(&models.Room{}).Where("id = ?", item.ID).Update("public_id", models.NewPublicID()).Error; err != nil {
			return err
		}
	}
	return nil
}

func backfillJoinRequestPublicIDs(db *gorm.DB) error {
	var items []models.JoinRequest
	if err := db.Where("public_id IS NULL").Find(&items).Error; err != nil {
		return err
	}
	for _, item := range items {
		if err := db.Model(&models.JoinRequest{}).Where("id = ?", item.ID).Update("public_id", models.NewPublicID()).Error; err != nil {
			return err
		}
	}
	return nil
}

func backfillRoomReservationPublicIDs(db *gorm.DB) error {
	var items []models.RoomReservation
	if err := db.Where("public_id IS NULL").Find(&items).Error; err != nil {
		return err
	}
	for _, item := range items {
		if err := db.Model(&models.RoomReservation{}).Where("id = ?", item.ID).Update("public_id", models.NewPublicID()).Error; err != nil {
			return err
		}
	}
	return nil
}

func backfillNotificationPublicIDs(db *gorm.DB) error {
	var items []models.Notification
	if err := db.Where("public_id IS NULL").Find(&items).Error; err != nil {
		return err
	}
	for _, item := range items {
		if err := db.Model(&models.Notification{}).Where("id = ?", item.ID).Update("public_id", models.NewPublicID()).Error; err != nil {
			return err
		}
	}
	return nil
}
