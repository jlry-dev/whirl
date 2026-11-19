package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jlry-dev/whirl/internal/config"
	"github.com/jlry-dev/whirl/internal/handler"
	"github.com/jlry-dev/whirl/internal/middleware"
	"github.com/jlry-dev/whirl/internal/repository"
	"github.com/jlry-dev/whirl/internal/service"
	"github.com/joho/godotenv"
)

// shutdownTimeout is how long we wait for in-flight HTTP requests before
// forcing the process down. Keep this below the orchestrator's kill grace.
const shutdownTimeout = 15 * time.Second

func newHTTPServer(addr string, handler http.Handler, logger *slog.Logger) *http.Server {
	return &http.Server{
		Addr:    addr,
		Handler: handler,

		// Headers are small, so 5s is already generous and blocks Slowloris.
		ReadHeaderTimeout: 5 * time.Second,

		// Bodies are small JSON, except avatar uploads. The frontend already
		// gives up at 30s, so ayaw ta pag-stricter sa client.
		ReadTimeout: 30 * time.Second,

		// The write clock starts before the handler runs, and avatar handling
		// includes a Cloudinary upload. 60s bounds a stuck response without
		// cutting off a slow but valid upload.
		WriteTimeout: 60 * time.Second,

		// Close idle keep-alive sockets. Browsers reopen them transparently.
		IdleTimeout: 60 * time.Second,

		ErrorLog: slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}
}

func main() {
	if _, err := os.Stat(".env"); err == nil {
		if err := godotenv.Load(); err != nil {
			log.Fatal("error loading .env file")
		}
	}

	srvConfig, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	dbPool, err := config.InitDB()
	if err != nil {
		log.Fatal(err)
	}

	// Repository
	userRepository := repository.NewUserRepository()
	avatarRepository := repository.NewAvatarRepository()
	countryRepository := repository.NewCountryRepository()
	friendshipRepository := repository.NewFriendshipRepository()
	messageRepository := repository.NewMessageRepository()

	// Services
	authSrv := service.NewAuthService(srvConfig.Validate, userRepository, countryRepository, dbPool, srvConfig.JWTKey)
	userSrv, err := service.NewUserService(srvConfig.Logger, userRepository, avatarRepository, dbPool)
	if err != nil {
		log.Fatal(err)
	}
	frSrv := service.NewFriendshipService(*srvConfig.Validate, srvConfig.Logger, friendshipRepository, &userRepository, dbPool)
	msgSrv := service.NewMessageService(srvConfig.Logger, messageRepository, dbPool)

	hub := handler.NewHub(frSrv, msgSrv, srvConfig.Logger)
	go hub.Run() // Start Hub work

	// Handler
	rspHandler := handler.NewResponseHandler(srvConfig.Logger)
	authHandlr := handler.NewAuthHandler(authSrv, rspHandler, srvConfig.Logger)
	userHandlr := handler.NewUserHandler(userSrv, srvConfig.Logger)
	ticketStore := handler.NewTicketStore(handler.TicketTTL)
	chatHandlr := handler.NewChatHandler(srvConfig.Logger, rspHandler, hub, srvConfig.Origins, ticketStore)
	frHandlr := handler.NewFriendshipHandler(srvConfig.Logger, rspHandler, frSrv)
	msgHandlr := handler.NewMessageHandler(msgSrv, rspHandler, srvConfig.Logger)

	// Middleware
	m := middleware.NewMiddleware(rspHandler, srvConfig.Logger, srvConfig.Origins, ticketStore, srvConfig.JWTKey)

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

	mux.HandleFunc("GET /messages/{id}", m.Authenticator(msgHandlr.RetrieveMessages))

	// Chat Matcher Worker
	mux.HandleFunc("POST /websocket/ticket", m.Authenticator(chatHandlr.IssueTicket))
	mux.HandleFunc("/websocket/connect", m.Authenticator(chatHandlr.SocketConnect))

	// Add middlewares to every request
	multiplexer := m.CorsMiddleware(mux)

	srv := newHTTPServer(srvConfig.Address, multiplexer, srvConfig.Logger)

	// SIGINT/SIGTERM starts a drain instead of killing connections mid-write.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	srvConfig.Logger.Info("server started", slog.String("address", srv.Addr))

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			srvConfig.Logger.Error("server stopped", slog.Any("error", err))
			os.Exit(1)
		}
	case <-ctx.Done():
		// Ibalik ang default handler, so a second Ctrl-C kills immediately.
		stop()
		srvConfig.Logger.Info("shutdown signal received")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			srvConfig.Logger.Error("graceful shutdown timed out", slog.Any("error", err))
			hub.Close()

			// Don't block on hung handlers or pgxpool.Close past the grace period.
			os.Exit(1)
		}
	}

	// Shutdown doesn't touch hijacked WebSocket connections, so close them here.
	hub.Close()
	dbPool.Close()
}
