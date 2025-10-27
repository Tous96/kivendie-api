package main

import (
	"context" // <--- AJOUTÉ
	"fmt"
	"log"
	"net/http"
	"os"

	//"github.com/joho/godotenv"
	"github.com/rs/cors" // NOUVEAU : Importer la bibliothèque CORS

	"kivendi-backend/config"
	"kivendi-backend/handlers"
	jobs "kivendi-backend/job"
	"kivendi-backend/routes"
	"kivendi-backend/services" // <--- AJOUTÉ
)

func main() {
	// Charge les variables d'environnement depuis le fichier .env
	//err := godotenv.Load()
	//if err != nil {
		//log.Fatal("Erreur lors du chargement du fichier .env")
	//}

	// Initialisez le service AWS
	handlers.InitAWSService()

	// ✅ CORRECTION DE L'INITIALISATION DU SERVICE PUSH
	// Appel correct à la fonction dans le package 'services'
	if err := services.InitPushService(context.Background()); err != nil {
		log.Fatalf("Échec de l'initialisation du service de Push: %v", err)
	}

	config.InitKKiaPay()
	log.Println("KKiaPay initialisé")
	// Définit la clé JWT dans le package handlers
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("Variable d'environnement 'JWT_SECRET' non définie")
	}
	handlers.SetJWTKey(jwtSecret)

	// Initialise la connexion à la base de données
	config.InitDB()

	// 👇 NOUVELLE INITIALISATION: Google OAuth 👇
	// Initialiser Google OAuth avec les credentials du fichier .env
	googleClientID := os.Getenv("GOOGLE_CLIENT_ID")
	googleClientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	googleRedirectURL := os.Getenv("GOOGLE_REDIRECT_URL")

	if googleClientID == "" || googleClientSecret == "" || googleRedirectURL == "" {
		log.Println("⚠️ 	ATTENTION: Les credentials Google OAuth ne sont pas configurés")
		log.Println("⚠️ 	L'authentification Google ne sera pas disponible")
	} else {
		handlers.InitGoogleOAuth(googleClientID, googleClientSecret, googleRedirectURL)
		log.Println("✅ Google OAuth initialisé avec succès")
	}
	// 👆 FIN DE L'INITIALISATION GOOGLE OAUTH 👆

	// Démarrer le job de nettoyage des boosts expirés
	jobs.StartBoostCleanupJob()
	// Configure le routeur
	router := routes.SetupRoutes()

	// NOUVEAU : Configuration du middleware CORS
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173", "https://votre-domaine-frontend.com"}, // IMPORTANT: Mettez ici l'URL de votre frontend
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		AllowCredentials: true,
		Debug:            true, // Active les logs de débogage pour CORS (utile pendant le développement)
	})

	// NOUVEAU : Appliquer le middleware CORS à votre routeur
	handler := c.Handler(router)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Utilise le port 8080 par défaut si non spécifié dans le .env
	}

	fmt.Printf("Serveur démarré sur le port %s...\n", port)
	// MODIFIÉ : Utiliser le handler avec CORS au lieu du routeur seul
	log.Fatal(http.ListenAndServe(":"+port, handler))
}
