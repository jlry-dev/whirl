package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/jlry-dev/whirl/internal/config"
	"github.com/jlry-dev/whirl/internal/handler"
	"github.com/jlry-dev/whirl/internal/middleware"
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
	friendshipRepository := repository.NewFriendshipRepository()
	messageRepository := repository.NewMessageRepository()

	// Services
	authSrv := service.NewAuthService(srvConfig.Validate, userRepository, countryRepository, dbPool)
	userSrv := service.NewUserService(srvConfig.Logger, userRepository, avatarRepository, dbPool)
	frSrv := service.NewFriendshipService(*srvConfig.Validate, srvConfig.Logger, friendshipRepository, &userRepository, dbPool)
	msgSrv := service.NewMessageService(srvConfig.Logger, messageRepository, dbPool)

	hub := handler.NewHub(frSrv, srvConfig.Logger)
	go hub.Run() // Start Hub work

	// Handler
	rspHandler := handler.NewResponseHandler(srvConfig.Logger)
	authHandlr := handler.NewAuthHandler(authSrv, rspHandler, srvConfig.Logger)
	userHandlr := handler.NewUserHandler(userSrv, srvConfig.Logger)
	chatHandlr := handler.NewChatHandler(srvConfig.Logger, rspHandler, hub)
	frHandlr := handler.NewFriendshipHandler(srvConfig.Logger, rspHandler, frSrv)
	msgHandlr := handler.NewMessageHandler(msgSrv, rspHandler, srvConfig.Logger)

	// Middleware
	m := middleware.NewMiddleware(rspHandler, srvConfig.Logger)

	// Multiplexer
	mux := http.NewServeMux()

	// Auth
	mux.HandleFunc("POST /auth/register", authHandlr.RegisterHandler)
	mux.HandleFunc("POST /auth/login", authHandlr.LoginHandler)

	// User
	mux.HandleFunc("POST /user/avatar", m.Authenticator(userHandlr.UpdateAvatar))

	// Friendship
	mux.HandleFunc("DELETE /friend", m.Authenticator(frHandlr.RemoveFriend))
	mux.HandleFunc("PUT /friend", m.Authenticator(frHandlr.UpdateFriendshipStatus))
	mux.HandleFunc("GET /friends", m.Authenticator(frHandlr.RetrieveFriends))

	mux.HandleFunc("GET /messages/:id", m.Authenticator(msgHandlr.RetrieveMessages))

	// Chat Matcher Worker
	mux.HandleFunc("/websocket/connect", m.Authenticator(chatHandlr.SocketConnect))

	srv_addr := os.Getenv("SERVER_ADDRESS")
	srv := http.Server{
		Handler:  mux,
		Addr:     srv_addr,
		ErrorLog: slog.NewLogLogger(srvConfig.Logger.Handler(), slog.LevelError),
	}

	log.Println("Server started at port: ", srv_addr)
	log.Fatal(srv.ListenAndServe())
}
