package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	banki "banki"
	"banki/internal/config"
	"banki/internal/database"
	"banki/internal/handlers"
	"banki/internal/middleware"
	"banki/internal/models"
	"banki/internal/templateutil"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

func methodOverride(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			if m := r.FormValue("_method"); m != "" {
				r.Method = m
			}
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	cfg := config.Load()

	if err := os.MkdirAll(cfg.UploadsDir, 0o755); err != nil {
		log.Fatalf("Failed to create uploads dir: %v", err)
	}

	middleware.InitSessions(cfg.SessionSecret)

	db, err := database.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	if err := database.Migrate(db); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	if err := models.EnsureDefaultUser(db, cfg.DefaultUser, cfg.DefaultPass); err != nil {
		log.Fatalf("Failed to create default user: %v", err)
	}

	if err := templateutil.Init(banki.TemplatesFS); err != nil {
		log.Fatalf("Failed to init templates: %v", err)
	}

	// Handlers
	authH := &handlers.AuthHandler{DB: db}
	dashH := &handlers.DashboardHandler{DB: db}
	banksH := &handlers.BanksHandler{DB: db, UploadsDir: cfg.UploadsDir}
	membersH := &handlers.FamilyMembersHandler{DB: db}
	depositsH := &handlers.DepositsHandler{DB: db}
	statsH := &handlers.StatsHandler{DB: db}
	walletsH := &handlers.WalletsHandler{DB: db}

	// Router
	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(securityHeaders)
	r.Use(methodOverride)

	// Static files
	fileServer := http.FileServer(http.FS(banki.StaticFS))
	r.Handle("/static/*", fileServer)

	// Uploaded files (logos etc.)
	uploadsServer := http.StripPrefix("/uploads/", http.FileServer(http.Dir(cfg.UploadsDir)))
	r.Handle("/uploads/*", uploadsServer)

	// Public routes
	r.Get("/login", authH.LoginPage)
	r.Post("/login", authH.LoginPage)

	// Protected routes — all authenticated users
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireAuth)
		r.Post("/logout", authH.Logout)
		r.Get("/", dashH.Index)

		// Deposits — accessible to all, filtering in handlers
		r.Get("/deposits", depositsH.Index)
		r.Get("/deposits/list", depositsH.List)
		r.Post("/deposits", depositsH.Create)
		r.Get("/deposits/{id}/edit", depositsH.EditForm)
		r.Put("/deposits/{id}", depositsH.Update)
		r.Post("/deposits/{id}/close", depositsH.Close)

		// Wallets — list and view for all, CUD admin-only
		r.Get("/wallets", walletsH.Index)
		r.Get("/wallets/{id}", walletsH.View)

		// Stats & ASV
		r.Get("/stats", statsH.Index)
		r.Get("/api/asv-check", statsH.ASVCheck)

		// Admin-only routes
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAdmin)

			// Banks
			r.Get("/banks", banksH.Index)
			r.Post("/banks", banksH.Create)
			r.Get("/banks/{id}/edit", banksH.EditForm)
			r.Put("/banks/{id}", banksH.Update)
			r.Delete("/banks/{id}", banksH.Delete)

			// Family members
			r.Get("/members", membersH.Index)
			r.Post("/members", membersH.Create)
			r.Get("/members/{id}/edit", membersH.EditForm)
			r.Put("/members/{id}", membersH.Update)
			r.Delete("/members/{id}", membersH.Delete)

			// Wallet management
			r.Post("/wallets", walletsH.Create)
			r.Get("/wallets/{id}/edit", walletsH.EditForm)
			r.Put("/wallets/{id}", walletsH.Update)
			r.Delete("/wallets/{id}", walletsH.Delete)
		})
	})

	fmt.Printf("Banki listening on %s\n", cfg.ListenAddr)
	if err := http.ListenAndServe(cfg.ListenAddr, r); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
