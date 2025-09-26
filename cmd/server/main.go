package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/jlry-dev/whirl/internal/config"
	"github.com/jlry-dev/whirl/internal/handler"
	"github.com/jlry-dev/whirl/internal/repository"
	"github.com/jlry-dev/whirl/internal/service"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("failed to load env.")
	}

	srvConfig := config.Load()

	dbPool := config.InitDB()

	// Repository
	userRepository := repository.NewUserRepository()
	avatarRepository := repository.NewAvatarRepository()
	countryRepository := repository.NewCountryRepository()

	// Services
	authSrv := service.NewAuthService(srvConfig.Validate, userRepository, countryRepository, dbPool)
	userSrv := service.NewUserService(srvConfig.Logger, userRepository, avatarRepository, dbPool)

	// Handler
	rspHandler := handler.NewResponseHandler(srvConfig.Logger)
	authHandlr := handler.NewAuthHandler(authSrv, rspHandler, srvConfig.Logger)
	userHandlr := handler.NewUserHandler(userSrv, srvConfig.Logger)

	// Multiplexer
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/register", authHandlr.RegisterHandler)

	// User handler
	mux.HandleFunc("POST /user/avatar", userHandlr.UpdateAvatar)

	srv_addr := os.Getenv("SERVER_ADDRESS")
	srv := http.Server{
		Handler:  mux,
		Addr:     srv_addr,
		ErrorLog: slog.NewLogLogger(srvConfig.Logger.Handler(), slog.LevelError),
	}

	log.Println("Server started at port: ", srv_addr)
	log.Fatal(srv.ListenAndServe())
}
