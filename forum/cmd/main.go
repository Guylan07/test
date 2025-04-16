package main

import (
	"flag"
	"fmt"
	"forum/internal/database"
	"forum/internal/handlers"
	"forum/internal/middleware"
	"forum/internal/models"
	"forum/internal/server"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"
)

// Ajoute des fonctions personnalisées aux templates et configure les variables d'environnement
func init() {
    // Configuration des variables d'environnement pour OAuth
    if os.Getenv("GOOGLE_CLIENT_ID") == "" {
        os.Setenv("GOOGLE_CLIENT_ID", "fake-client-id")
        os.Setenv("GOOGLE_CLIENT_SECRET", "fake-client-secret")
    }
    
    if os.Getenv("GITHUB_CLIENT_ID") == "" {
        os.Setenv("GITHUB_CLIENT_ID", "fake-client-id")
        os.Setenv("GITHUB_CLIENT_SECRET", "fake-client-secret")
    }
    
    // Crée un nouveau template avec les fonctions nécessaires pour la pagination
    // Ces fonctions seront disponibles dans tous les templates
    template.New("").Funcs(template.FuncMap{
        "add": func(a, b int) int {
            return a + b
        },
        "subtract": func(a, b int) int {
            return a - b
        },
        "sequence": func(start, end int) []int {
            var result []int
            for i := start; i <= end; i++ {
                result = append(result, i)
            }
            return result
        },
    })
}

func main() {
	// Définir et lire les options de ligne de commande
	var (
		dbPath      = flag.String("db", "./forum.db", "Chemin vers la base de données SQLite")
		httpPort    = flag.Int("http", 8085, "Port HTTP")
		httpsPort   = flag.Int("https", 8443, "Port HTTPS")
		certDir     = flag.String("certs", "./certs", "Dossier des certificats SSL")
		domain      = flag.String("domain", "localhost", "Nom de domaine pour HTTPS")
		dev         = flag.Bool("dev", true, "Mode développement (certificat auto-signé)")
		uploadDir   = flag.String("uploads", "./static/uploads", "Dossier pour les uploads")
	)
	flag.Parse()

	// Créer les dossiers nécessaires s'ils n'existent pas
	os.MkdirAll(*certDir, 0755)
	os.MkdirAll(*uploadDir, 0755)
	
	// On initialise la base de données au démarrage du programme
	err := database.InitDB(*dbPath)
	if err != nil {
		// Si on n'arrive pas à initialiser la base de données, on arrête tout
		log.Fatalf("Error initializing database: %v", err)
	}
	// On s'assure que la base de données sera fermée proprement à la fin du programme
	defer database.CloseDB()
	
	// Initialiser les tables supplémentaires pour les fonctionnalités étendues
	if err := models.InitImageTables(); err != nil {
		log.Fatalf("Error initializing image tables: %v", err)
	}
	
	if err := models.ModerationQueueInit(); err != nil {
		log.Fatalf("Error initializing moderation tables: %v", err)
	}
	
	// On crée un nouveau routeur pour gérer les différentes adresses du site
	mux := http.NewServeMux()
	
	// On configure le serveur pour qu'il puisse fournir des fichiers statiques (images, CSS, JavaScript)
	fs := http.FileServer(http.Dir("./static"))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))
	
	// On configure les routes pour l'authentification
	mux.HandleFunc("/register", handlers.RegisterHandler)
	mux.HandleFunc("/login", handlers.LoginHandler)
	mux.HandleFunc("/logout", handlers.LogoutHandler)
	
	// Routes pour l'authentification OAuth
	mux.HandleFunc("/auth/google", handlers.GoogleLoginHandler)
	mux.HandleFunc("/auth/google/callback", handlers.GoogleCallbackHandler)
	mux.HandleFunc("/auth/github", handlers.GitHubLoginHandler)
	mux.HandleFunc("/auth/github/callback", handlers.GitHubCallbackHandler)
	
	// Routes pour les pages principales
	mux.HandleFunc("/", handlers.HomeHandler)
	
	// Routes pour les posts
	mux.HandleFunc("/post/create", handlers.CreatePostHandler)
	mux.HandleFunc("/post/", handlers.ViewPostHandler)
	mux.HandleFunc("/post/edit/", handlers.EditPostHandler)
	mux.HandleFunc("/post/delete/", handlers.DeletePostHandler)
	mux.HandleFunc("/post/react", handlers.ReactToPostHandler)

	mux.HandleFunc("/profile", handlers.ProfileHandler)
	
	// Routes pour les commentaires
	mux.HandleFunc("/comment/create", handlers.CreateCommentHandler)
	mux.HandleFunc("/comment/edit", handlers.EditCommentHandler)
	mux.HandleFunc("/comment/delete/", handlers.DeleteCommentHandler)
	mux.HandleFunc("/comment/react", handlers.ReactToCommentHandler)
	
	// Routes pour le téléchargement et la gestion des images
	mux.HandleFunc("/upload/image", handlers.UploadImageHandler)
	mux.HandleFunc("/image/", handlers.GetImageHandler)
	
	// Routes pour la modération
	moderatorMux := http.NewServeMux()
	moderatorMux.HandleFunc("/mod/pending", handlers.ListPendingPostsHandler)
	moderatorMux.HandleFunc("/mod/approve/", handlers.ApprovePostHandler)
	moderatorMux.HandleFunc("/mod/reject/", handlers.RejectPostHandler)
	
	// Routes pour l'administration
	adminMux := http.NewServeMux()
	adminMux.HandleFunc("/admin/reports", handlers.ListReportsHandler)
	adminMux.HandleFunc("/admin/report/handle/", handlers.HandleReportHandler)
	adminMux.HandleFunc("/admin/users", handlers.ListUsersHandler)
	adminMux.HandleFunc("/admin/user/role/", handlers.UpdateUserRoleHandler)
	
	// Application des middleware de sécurité pour chaque niveau d'accès
	moderatorHandler := middleware.RequireRoleMiddleware(moderatorMux, "moderator")
	adminHandler := middleware.RequireRoleMiddleware(adminMux, "admin")
	
	// Ajouter les routes protégées au routeur principal
	mux.Handle("/mod/", moderatorHandler)
	mux.Handle("/admin/", adminHandler)
	
	// Créer un rate limiter
	rateLimiter := middleware.NewRateLimiter(100, time.Minute)
	
	// Appliquer les middleware dans l'ordre
	handler := middleware.LoggingMiddleware(mux)
	handler = rateLimiter.Middleware(handler)
	handler = middleware.AuthMiddleware(handler)
	
	// Configuration HTTPS
	httpsConfig := server.HTTPSConfig{
		Domain:      *domain,
		CertPath:    *certDir,
		Development: *dev,
	}
	
	// Configurer le serveur HTTPS
	httpsServer := server.ConfigureHTTPS(handler, httpsConfig)
	
	// Si en mode développement, générer/utiliser un certificat auto-signé
	if *dev {
		certFile, keyFile, err := server.GenerateSelfSignedCert(*certDir)
		if err != nil {
			log.Printf("Warning: Failed to set up self-signed certificate: %v", err)
			log.Printf("Running in HTTP-only mode")
			
			// Démarrer le serveur HTTP
			log.Printf("Starting HTTP server on port %d...", *httpPort)
			log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *httpPort), handler))
		} else {
			// Démarrer le serveur HTTPS avec le certificat auto-signé
			log.Printf("Starting HTTPS server on port %d with self-signed certificate...", *httpsPort)
			go func() {
				log.Printf("Starting HTTP redirect server on port %d...", *httpPort)
				redirectHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					target := fmt.Sprintf("https://%s:%d%s", r.Host, *httpsPort, r.URL.Path)
					if len(r.URL.RawQuery) > 0 {
						target += "?" + r.URL.RawQuery
					}
					http.Redirect(w, r, target, http.StatusMovedPermanently)
				})
				log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *httpPort), redirectHandler))
			}()
			
			log.Fatal(httpsServer.ListenAndServeTLS(certFile, keyFile))
		}
	} else {
		// En production, le serveur sera démarré avec la configuration Let's Encrypt
		log.Printf("Starting HTTPS server with Let's Encrypt certificates...")
		log.Fatal(httpsServer.ListenAndServeTLS("", ""))
	}
}