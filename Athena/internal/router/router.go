package router

import (
	"athena/internal/config"
	"athena/internal/database"
	"athena/internal/handlers"
	"athena/internal/middleware"
	"crypto/rsa"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
)

type HandlerDependencies struct {
	UserRepo    database.UserRepository
	ProjectRepo database.ProjectRepository
	RouteRepo   database.RouteRepository
	TokenRepo   database.TokenRepository
	DBPinger    database.DBPinger
	PrivateKey  *rsa.PrivateKey
	PublicKey   *rsa.PublicKey
	Config      *config.Config
	AdminHandlers *handlers.AdminHandlers
}

func SetupRouter(deps HandlerDependencies) http.Handler {
	// Handler initialisieren
	authHandlers := handlers.NewAuthHandlers(deps.UserRepo, deps.ProjectRepo, deps.TokenRepo, deps.PrivateKey, deps.Config.JWTAccessTokenTTL, deps.Config.JWTRefreshTokenTTL, deps.Config,)
	otpHandlers := handlers.NewOTPHandlers(deps.UserRepo, deps.ProjectRepo, deps.TokenRepo, deps.Config.OTPIssuerName, deps.PrivateKey, deps.Config.JWTAccessTokenTTL, deps.Config.JWTRefreshTokenTTL)
	tokenHandlers := handlers.NewTokenHandlers(deps.UserRepo, deps.ProjectRepo, deps.TokenRepo, deps.PrivateKey, deps.Config.JWTAccessTokenTTL, deps.Config.JWTRefreshTokenTTL)
	userHandler := handlers.NewUserHandlers(deps.UserRepo, deps.ProjectRepo)
	projectHandlers := handlers.NewProjectHandlers(deps.ProjectRepo, deps.RouteRepo, deps.UserRepo) // (Mit der Korrektur aus Schritt 3)
	internalHandlers := handlers.NewInternalHandlers(deps.RouteRepo, deps.ProjectRepo)

	r := chi.NewRouter()

	// Globale Middlewares
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(httprate.LimitAll(100, 1*time.Second))
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(middleware.SecurityHeaders)
	r.Use(middleware.AuditContext)

	// Health Check & Root
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Auth Service ist online!"))
	})
	r.Get("/health", handlers.HealthCheckHandler(deps.DBPinger))

	// --- Routen-Definitionen ---
	r.Route("/auth", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(middleware.PermissiveProjectIDInjector)
			r.Post("/login", authHandlers.LoginHandler)
			r.Post("/register", authHandlers.RegisterHandler)
		})

		r.Post("/refresh", tokenHandlers.RefreshHandler)
		r.Post("/login/admin-otp", deps.AdminHandlers.LoginGlobalOTPHandler)

		r.Group(func(r chi.Router) {
			r.Use(middleware.ProjectIDValidator)
			r.Post("/login/otp", otpHandlers.LoginOTPHandler)
			r.Post("/login/otp-standalone", otpHandlers.LoginOTPStandaloneHandler)
		})
	})

	r.Route("/users", func(r chi.Router) {
		r.Use(middleware.Authenticator(deps.PublicKey))
		r.Use(middleware.ProjectIDValidator)
		r.Get("/me", userHandler.GetCurrentUserHandler)
		r.Put("/me", userHandler.UpdateCurrentUserHandler)
	})

	r.Route("/auth/otp", func(r chi.Router) {
		r.Use(middleware.Authenticator(deps.PublicKey))
		r.Use(middleware.ProjectIDValidator)

		r.Post("/setup", otpHandlers.SetupOTPHandler)
		r.Post("/verify", otpHandlers.VerifyOTPHandler)
		r.Post("/disable", otpHandlers.DisableOTPHandler)
	})

	r.Route("/admin/otp", func(r chi.Router) {
        r.Use(middleware.Authenticator(deps.PublicKey))
        r.Use(deps.AdminHandlers.OnlyAdmin) // Sichert alle Routen in dieser Gruppe

		r.Get("/profile", deps.AdminHandlers.GetGlobalProfileHandler)

        r.Post("/setup", deps.AdminHandlers.SetupGlobalOTPHandler)
        r.Post("/verify", deps.AdminHandlers.VerifyGlobalOTPHandler)
        r.Post("/disable", deps.AdminHandlers.DisableGlobalOTPHandler)
    })

	r.Route("/projects", func(r chi.Router) {
		r.Use(middleware.Authenticator(deps.PublicKey))
		r.Post("/", projectHandlers.CreateProjectHandler)
		r.Get("/", projectHandlers.GetMyProjectsHandler)

		r.Route("/{projectID}", func(r chi.Router) {
			r.Get("/", projectHandlers.GetProjectDetailsHandler)
			r.Put("/settings", projectHandlers.UpdateProjectSettingsHandler)

			// Benutzer-Management
			r.Post("/users", projectHandlers.AddUserToProjectHandler)
			r.Get("/users", projectHandlers.GetProjectUsersHandler)
			r.Put("/users/{userID}", projectHandlers.UpdateUserRolesHandler)
			r.Delete("/users/{userID}", projectHandlers.RemoveUserFromProjectHandler)

			// Routen-Management
			r.Post("/routes", projectHandlers.CreateProjectRouteHandler)
			r.Get("/routes", projectHandlers.GetProjectRoutesHandler)
			
			r.Delete("/routes/{routeID}", projectHandlers.DeleteProjectRouteHandler)
			r.Put("/routes/{routeID}", projectHandlers.UpdateProjectRouteHandler)
		})
	})

	r.Route("/internal/v1", func(r chi.Router) {
		r.Use(middleware.InternalAuth)
		r.Get("/routes/config", internalHandlers.GetAllRoutesConfigHandler)
		r.Get("/context-map", internalHandlers.GetContextMapHandler)
	})

	return r
}