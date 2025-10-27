package handlers

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	// ✅ IMPORT AJOUTÉ
	"google.golang.org/api/idtoken"

	"kivendi-backend/config"
	"kivendi-backend/models"
)

var (
	googleOauthConfig *oauth2.Config
	// Note: 'oauthStateString' n'est utilisé que pour le flux web (GoogleLoginHandler)
	// oauthStateString  = generateStateOauthCookie()
)

// InitGoogleOAuth initialise la configuration Google OAuth
// À appeler depuis main.go au démarrage de l'application
func InitGoogleOAuth(clientID, clientSecret, redirectURL string) {
	googleOauthConfig = &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL, // Utilisé pour le flux web
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}
	log.Println("✅ Google OAuth configuré avec succès")
}

// generateStateOauthCookie génère un état aléatoire pour la sécurité OAuth (flux web)
func generateStateOauthCookie() string {
	b := make([]byte, 32)
	rand.Read(b)
	state := base64.URLEncoding.EncodeToString(b)
	return state
}

// =================================================================================
// HANDLERS POUR LE FLUX WEB (Navigateur)
// =================================================================================

// GoogleLoginHandler redirige l'utilisateur vers la page de connexion Google (Flux Web)
func GoogleLoginHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("🔵 Redirection vers Google OAuth (Flux Web)...")

	if googleOauthConfig == nil {
		log.Println("❌ Erreur: Google OAuth n'est pas configuré")
		http.Error(w, "Service d'authentification Google non disponible", http.StatusInternalServerError)
		return
	}

	// Générer un nouvel état pour cette session
	state := generateStateOauthCookie()

	// Stocker l'état dans un cookie pour vérification ultérieure
	cookie := &http.Cookie{
		Name:     "oauthstate",
		Value:    state,
		Expires:  time.Now().Add(10 * time.Minute),
		HttpOnly: true,
		Secure:   true, // À mettre à false en développement sans HTTPS
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	}
	http.SetCookie(w, cookie)

	// Générer l'URL d'autorisation Google
	url := googleOauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)

	log.Printf("🔗 URL de redirection Google: %s", url)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// GoogleUserInfo structure pour les informations de l'utilisateur Google (Flux Web)
type GoogleUserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
	Locale        string `json:"locale"`
}

// GoogleCallbackHandler gère le retour de Google après authentification (Flux Web)
func GoogleCallbackHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("🔵 Callback Google OAuth reçu (Flux Web)...")

	// Vérifier l'état OAuth pour prévenir les attaques CSRF
	stateCookie, err := r.Cookie("oauthstate")
	if err != nil {
		log.Printf("❌ Cookie d'état manquant: %v", err)
		http.Error(w, "État de session invalide", http.StatusBadRequest)
		return
	}

	if r.FormValue("state") != stateCookie.Value {
		log.Println("❌ État OAuth invalide - possible attaque CSRF")
		http.Error(w, "État OAuth invalide", http.StatusBadRequest)
		return
	}

	// Supprimer le cookie d'état
	http.SetCookie(w, &http.Cookie{
		Name:     "oauthstate",
		Value:    "",
		Expires:  time.Now().Add(-1 * time.Hour),
		HttpOnly: true,
		Path:     "/",
	})

	// Récupérer le code d'autorisation
	code := r.FormValue("code")
	if code == "" {
		log.Println("❌ Code d'autorisation manquant")
		http.Error(w, "Code d'autorisation manquant", http.StatusBadRequest)
		return
	}

	// Échanger le code contre un token
	token, err := googleOauthConfig.Exchange(context.Background(), code)
	if err != nil {
		log.Printf("❌ Erreur lors de l'échange du code: %v", err)
		http.Error(w, "Impossible d'échanger le code d'autorisation", http.StatusInternalServerError)
		return
	}

	// Récupérer les informations de l'utilisateur depuis Google
	userInfo, err := getUserInfoFromGoogle(token.AccessToken)
	if err != nil {
		log.Printf("❌ Erreur lors de la récupération des infos utilisateur: %v", err)
		http.Error(w, "Impossible de récupérer les informations utilisateur", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Infos utilisateur Google: Email=%s, Name=%s, GoogleID=%s",
		userInfo.Email, userInfo.Name, userInfo.ID)

	// Vérifier si l'email est vérifié chez Google
	if !userInfo.VerifiedEmail {
		log.Printf("❌ Email non vérifié chez Google: %s", userInfo.Email)
		http.Error(w, "Votre email Google n'est pas vérifié", http.StatusForbidden)
		return
	}

	// Chercher si l'utilisateur existe déjà
	var existingUser models.User
	var passwordHash sql.NullString
	var googleID sql.NullString

	err = config.DB.QueryRow(`
		SELECT id, first_name, last_name, email, password_hash, google_id, 
		       account_type, shop_name, avatar_url, is_verified, is_blocked, 
		       created_at, updated_at
		FROM users WHERE email = $1
	`, userInfo.Email).Scan(
		&existingUser.ID,
		&existingUser.FirstName,
		&existingUser.LastName,
		&existingUser.Email,
		&passwordHash,
		&googleID,
		&existingUser.AccountType,
		&existingUser.ShopName,
		&existingUser.AvatarURL,
		&existingUser.IsVerified,
		&existingUser.IsBlocked,
		&existingUser.CreatedAt,
		&existingUser.UpdatedAt,
	)

	// CAS 1: L'utilisateur existe déjà
	if err == nil {
		log.Printf("👤 Utilisateur existant trouvé: ID=%d, Email=%s", existingUser.ID, existingUser.Email)

		// RÈGLE IMPORTANTE: Si l'utilisateur a un mot de passe, il ne peut PAS se connecter avec Google
		if passwordHash.Valid && passwordHash.String != "" {
			log.Printf("⛔ Tentative de connexion Google bloquée - L'utilisateur %s a déjà un compte avec mot de passe", userInfo.Email)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)

			// ✅ LOG AJOUTÉ: Voir le message d'erreur envoyé
			responsePayload := map[string]interface{}{
				"error":   "Compte existant avec mot de passe",
				"message": "Vous avez déjà un compte avec un mot de passe. Veuillez vous connecter avec votre email et mot de passe. L'authentification Google n'est pas disponible pour votre compte.",
			}
			log.Printf("📤 [RESP-403] Envoi du message d'erreur: %v", responsePayload)
			json.NewEncoder(w).Encode(responsePayload)
			return
		}

		// Vérifier si le compte est bloqué
		if existingUser.IsBlocked {
			log.Printf("⛔ Compte bloqué: %s", userInfo.Email)
			// ✅ LOG AJOUTÉ: Voir le message d'erreur envoyé
			log.Printf("📤 [RESP-403] Envoi du message d'erreur: Compte bloqué.")
			http.Error(w, "Votre compte a été bloqué. Veuillez contacter le support.", http.StatusForbidden)
			return
		}

		// Si l'utilisateur n'a pas encore de google_id, on l'ajoute
		if !googleID.Valid || googleID.String == "" {
			log.Printf("🔄 Ajout du Google ID pour l'utilisateur existant: %s", userInfo.Email)

			_, err = config.DB.Exec(`
				UPDATE users 
				SET google_id = $1, 
				    is_verified = true,
				    avatar_url = COALESCE(avatar_url, $2),
				    updated_at = $3
				WHERE id = $4
			`, userInfo.ID, sql.NullString{String: userInfo.Picture, Valid: userInfo.Picture != ""}, time.Now(), existingUser.ID)

			if err != nil {
				log.Printf("❌ Erreur lors de la mise à jour du Google ID: %v", err)
				http.Error(w, "Erreur lors de la mise à jour du compte", http.StatusInternalServerError)
				return
			}
		}

		// Générer les tokens JWT
		accessToken, refreshToken, expiresIn, err := generateTokensForUser(existingUser.Email, existingUser.ID)
		if err != nil {
			log.Printf("❌ Erreur lors de la génération des tokens: %v", err)
			http.Error(w, "Erreur lors de la génération des tokens", http.StatusInternalServerError)
			return
		}

		log.Printf("✅ Connexion Google réussie pour l'utilisateur existant: %s", userInfo.Email)

		// Note: Ce flux est pour le WEB.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// ✅ LOG AJOUTÉ: Voir le message de succès envoyé
		responsePayload := map[string]interface{}{
			"message":      "Connexion Google réussie",
			"token":        accessToken,
			"refreshToken": refreshToken,
			"expiresIn":    expiresIn,
			"user":         existingUser.ToResponse(),
		}
		log.Printf("📤 [RESP-200] Envoi de la réponse de connexion: UserID: %d, ExpiresIn: %d", existingUser.ID, expiresIn)
		json.NewEncoder(w).Encode(responsePayload)
		return
	}

	// Si erreur autre que "pas de résultat"
	if err != sql.ErrNoRows {
		log.Printf("❌ Erreur de base de données: %v", err)
		http.Error(w, "Erreur de base de données", http.StatusInternalServerError)
		return
	}

	// CAS 2: Nouvel utilisateur - Créer un compte
	log.Printf("➕ Création d'un nouveau compte Google pour: %s", userInfo.Email)

	// Extraire le prénom et nom de famille
	firstName := userInfo.GivenName
	lastName := userInfo.FamilyName

	// Si GivenName et FamilyName ne sont pas disponibles, utiliser Name
	if firstName == "" && userInfo.Name != "" {
		firstName = userInfo.Name
	}
	if lastName == "" {
		lastName = "" // Peut être vide
	}

	// Vérifier que les champs nécessaires sont présents
	if firstName == "" {
		log.Println("❌ Prénom manquant dans les informations Google")
		http.Error(w, "Informations de profil incomplètes. Veuillez vous inscrire manuellement.", http.StatusBadRequest)
		return
	}

	// Insertion du nouvel utilisateur
	var newUserID int
	query := `
		INSERT INTO users (
			first_name, last_name, email, google_id, avatar_url,
			account_type, is_verified, is_blocked, 
			created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id
	`

	err = config.DB.QueryRow(
		query,
		firstName,
		lastName,
		userInfo.Email,
		userInfo.ID, // google_id
		sql.NullString{String: userInfo.Picture, Valid: userInfo.Picture != ""},
		"Personnel", // Type de compte par défaut (ou "Acheteur")
		true,        // is_verified (Google a déjà vérifié l'email)
		false,       // is_blocked
		time.Now(),
		time.Now(),
	).Scan(&newUserID)

	if err != nil {
		log.Printf("❌ Erreur lors de la création de l'utilisateur: %v", err)
		http.Error(w, "Erreur lors de la création du compte", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Nouveau compte Google créé - ID: %d, Email: %s", newUserID, userInfo.Email)

	// Créer l'objet utilisateur pour la réponse
	newUser := models.User{
		ID:          newUserID,
		FirstName:   firstName,
		LastName:    lastName,
		Email:       userInfo.Email,
		AccountType: "Personnel",
		AvatarURL:   sql.NullString{String: userInfo.Picture, Valid: userInfo.Picture != ""},
		IsVerified:  true,
		IsBlocked:   false,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Générer les tokens JWT
	accessToken, refreshToken, expiresIn, err := generateTokensForUser(newUser.Email, newUser.ID)
	if err != nil {
		log.Printf("❌ Erreur lors de la génération des tokens: %v", err)
		http.Error(w, "Erreur lors de la génération des tokens", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Inscription et connexion Google réussies pour: %s", userInfo.Email)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	// ✅ LOG AJOUTÉ: Voir le message de création envoyé
	responsePayload := map[string]interface{}{
		"message":      "Compte créé et connexion Google réussie",
		"token":        accessToken,
		"refreshToken": refreshToken,
		"expiresIn":    expiresIn,
		"user":         newUser.ToResponse(),
	}
	log.Printf("📤 [RESP-201] Envoi de la réponse de création: UserID: %d, ExpiresIn: %d", newUser.ID, expiresIn)
	json.NewEncoder(w).Encode(responsePayload)
}

// getUserInfoFromGoogle récupère les informations de l'utilisateur depuis l'API Google (Flux Web)
func getUserInfoFromGoogle(accessToken string) (*GoogleUserInfo, error) {
	// Construire la requête vers l'API Google UserInfo
	resp, err := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + accessToken)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de la requête vers Google API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("erreur API Google (status %d): %s", resp.StatusCode, string(body))
	}

	// Décoder la réponse JSON
	var userInfo GoogleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, fmt.Errorf("erreur lors du décodage des infos utilisateur: %w", err)
	}

	return &userInfo, nil
}

// =================================================================================
// HANDLER POUR LE FLUX MOBILE (Flutter)
// =================================================================================

// GoogleTokenRequest structure pour la requête de connexion mobile
type GoogleTokenRequest struct {
	Token string `json:"token"`
}

// GoogleMobileLoginHandler gère la connexion depuis une application mobile
func GoogleMobileLoginHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("🔵 Connexion Google Mobile reçue...")
	w.Header().Set("Content-Type", "application/json")

	// 1. Lire le token Google depuis le corps de la requête
	var req GoogleTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ Erreur de décodage JSON: %v", err)
		http.Error(w, "Requête invalide", http.StatusBadRequest)
		return
	}

	// ✅ LOG AJOUTÉ: Voir le token reçu (tronqué pour la sécurité)
	tokenDisplay := req.Token
	if len(tokenDisplay) > 20 {
		tokenDisplay = tokenDisplay[:20] + "..."
	}
	log.Printf("📄 [REQ] Requête reçue avec token: %s", tokenDisplay)

	if req.Token == "" {
		log.Println("❌ Token Google (idToken) manquant")
		http.Error(w, "Token manquant", http.StatusBadRequest)
		return
	}

	// 2. Vérifier le token Google (idToken)
	if googleOauthConfig == nil || googleOauthConfig.ClientID == "" {
		log.Println("❌ Erreur: Google OAuth n'est pas configuré (ClientID manquant)")
		http.Error(w, "Service d'authentification non configuré", http.StatusInternalServerError)
		return
	}

	// ✅ LOG AJOUTÉ: Indique le début de la validation
	log.Printf("🔍 [AUTH] Validation du token Google (idToken) pour ClientID: %s", googleOauthConfig.ClientID)

	payload, err := idtoken.Validate(context.Background(), req.Token, googleOauthConfig.ClientID)
	if err != nil {
		log.Printf("❌ [AUTH-ERR] Erreur de validation du token Google: %v", err)
		http.Error(w, "Token Google invalide", http.StatusUnauthorized)
		return
	}

	// 3. Extraire les informations de l'utilisateur du payload
	claims := payload.Claims
	email, _ := claims["email"].(string)
	verifiedEmail, _ := claims["email_verified"].(bool)
	name, _ := claims["name"].(string)
	givenName, _ := claims["given_name"].(string)
	familyName, _ := claims["family_name"].(string)
	picture, _ := claims["picture"].(string)
	googleID := payload.Subject // "sub" est l'ID utilisateur Google

	log.Printf("✅ [AUTH-OK] Token validé. Email: %s, GoogleID (sub): %s", email, googleID)

	// 4. Vérifier si l'email est vérifié chez Google
	if !verifiedEmail {
		log.Printf("🚫 [AUTH-FAIL] Email non vérifié chez Google: %s", email)
		http.Error(w, "Votre email Google n'est pas vérifié", http.StatusForbidden)
		return
	}

	// 5. Logique de recherche/création d'utilisateur
	var existingUser models.User
	var passwordHash sql.NullString
	var dbGoogleID sql.NullString

	// ✅ LOG AJOUTÉ: Indique la recherche en BDD
	log.Printf("🔍 [DB] Recherche de l'utilisateur: %s", email)

	err = config.DB.QueryRow(`
		SELECT id, first_name, last_name, email, password_hash, google_id, 
		       account_type, shop_name, avatar_url, is_verified, is_blocked, 
		       created_at, updated_at
		FROM users WHERE email = $1
	`, email).Scan(
		&existingUser.ID,
		&existingUser.FirstName,
		&existingUser.LastName,
		&existingUser.Email,
		&passwordHash,
		&dbGoogleID,
		&existingUser.AccountType,
		&existingUser.ShopName,
		&existingUser.AvatarURL,
		&existingUser.IsVerified,
		&existingUser.IsBlocked,
		&existingUser.CreatedAt,
		&existingUser.UpdatedAt,
	)

	// CAS 1: L'utilisateur existe déjà
	if err == nil {
		log.Printf("👤 [DB-OK] Utilisateur existant trouvé: ID=%d, Email=%s", existingUser.ID, existingUser.Email)

		// RÈGLE IMPORTANTE: Si l'utilisateur a un mot de passe, il ne peut PAS se connecter avec Google
		if passwordHash.Valid && passwordHash.String != "" {
			log.Printf("⛔ [AUTH-FAIL] Conflit: L'utilisateur %s a un compte avec mot de passe.", email)
			w.WriteHeader(http.StatusForbidden)

			// ✅ LOG AJOUTÉ: Voir le message d'erreur envoyé
			responsePayload := map[string]interface{}{
				"error":   "Compte existant avec mot de passe",
				"message": "Vous avez déjà un compte avec un mot de passe. Veuillez vous connecter avec votre email et mot de passe.",
			}
			log.Printf("📤 [RESP-403] Envoi du message d'erreur (conflit mot de passe): %v", responsePayload)
			json.NewEncoder(w).Encode(responsePayload)
			return
		}

		// Vérifier si le compte est bloqué
		if existingUser.IsBlocked {
			log.Printf("⛔ [AUTH-FAIL] Compte bloqué: %s", email)
			// ✅ LOG AJOUTÉ: Voir le message d'erreur envoyé
			log.Printf("📤 [RESP-403] Envoi du message d'erreur: Compte bloqué.")
			http.Error(w, "Votre compte a été bloqué. Veuillez contacter le support.", http.StatusForbidden)
			return
		}

		// Si l'utilisateur n'a pas encore de google_id, on l'ajoute
		if !dbGoogleID.Valid || dbGoogleID.String == "" {
			log.Printf("🔄 [DB] Ajout du Google ID (%s) pour l'utilisateur existant: %s", googleID, email)
			_, err = config.DB.Exec(`
				UPDATE users 
				SET google_id = $1, 
				    is_verified = true,
				    avatar_url = COALESCE(avatar_url, $2),
				    updated_at = $3
				WHERE id = $4
			`, googleID, sql.NullString{String: picture, Valid: picture != ""}, time.Now(), existingUser.ID)

			if err != nil {
				log.Printf("❌ [DB-ERR] Erreur lors de la mise à jour du Google ID: %v", err)
				http.Error(w, "Erreur lors de la mise à jour du compte", http.StatusInternalServerError)
				return
			}
		}

		// Générer les tokens JWT
		accessToken, refreshToken, expiresIn, err := generateTokensForUser(existingUser.Email, existingUser.ID)
		if err != nil {
			log.Printf("❌ [JWT-ERR] Erreur lors de la génération des tokens: %v", err)
			http.Error(w, "Erreur lors de la génération des tokens", http.StatusInternalServerError)
			return
		}

		log.Printf("✅ [AUTH-OK] Connexion Google (Mobile) réussie pour: %s", email)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// ✅ LOG AJOUTÉ: Voir le message de succès envoyé
		responsePayload := map[string]interface{}{
			"token":        accessToken,
			"refreshToken": refreshToken,
			"expiresIn":    expiresIn,
			"user":         existingUser.ToResponse(),
		}
		log.Printf("📤 [RESP-200] Envoi de la réponse de connexion: UserID: %d, ExpiresIn: %d", existingUser.ID, expiresIn)
		json.NewEncoder(w).Encode(responsePayload)
		return
	}

	// Si erreur autre que "pas de résultat"
	if err != sql.ErrNoRows {
		log.Printf("❌ [DB-ERR] Erreur de base de données inattendue: %v", err)
		// ✅ LOG AJOUTÉ: Voir le message d'erreur envoyé
		log.Printf("📤 [RESP-500] Envoi du message d'erreur: Erreur de base de données.")
		http.Error(w, "Erreur de base de données", http.StatusInternalServerError)
		return
	}

	// CAS 2: Nouvel utilisateur - Créer un compte
	log.Printf("➕ [DB] Nouvel utilisateur. Création du compte Google (Mobile) pour: %s", email)

	firstName := givenName
	lastName := familyName
	if firstName == "" && name != "" {
		firstName = name
	}

	if firstName == "" {
		log.Println("❌ [AUTH-FAIL] Prénom manquant dans les informations Google")
		http.Error(w, "Informations de profil incomplètes. Veuillez vous inscrire manuellement.", http.StatusBadRequest)
		return
	}

	// Insertion du nouvel utilisateur
	var newUserID int
	query := `
		INSERT INTO users (
			first_name, last_name, email, google_id, avatar_url,
			account_type, is_verified, is_blocked, 
			created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id
	`

	err = config.DB.QueryRow(
		query,
		firstName,
		lastName,
		email,
		googleID, // google_id
		sql.NullString{String: picture, Valid: picture != ""},
		"Personnel", // Type de compte par défaut (ou "Acheteur")
		true,        // is_verified (Google a déjà vérifié l'email)
		false,       // is_blocked
		time.Now(),
		time.Now(),
	).Scan(&newUserID)

	if err != nil {
		log.Printf("❌ [DB-ERR] Erreur lors de la création de l'utilisateur: %v", err)
		http.Error(w, "Erreur lors de la création du compte", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ [DB-OK] Nouveau compte Google créé - ID: %d, Email: %s", newUserID, email)

	// Créer l'objet utilisateur pour la réponse
	newUser := models.User{
		ID:          newUserID,
		FirstName:   firstName,
		LastName:    lastName,
		Email:       email,
		AccountType: "Personnel",
		AvatarURL:   sql.NullString{String: picture, Valid: picture != ""},
		IsVerified:  true,
		IsBlocked:   false,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Générer les tokens JWT
	accessToken, refreshToken, expiresIn, err := generateTokensForUser(newUser.Email, newUser.ID)
	if err != nil {
		log.Printf("❌ [JWT-ERR] Erreur lors de la génération des tokens: %v", err)
		http.Error(w, "Erreur lors de la génération des tokens", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ [AUTH-OK] Inscription et connexion Google (Mobile) réussies pour: %s", email)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	// ✅ LOG AJOUTÉ: Voir le message de création envoyé
	responsePayload := map[string]interface{}{
		"token":        accessToken,
		"refreshToken": refreshToken,
		"expiresIn":    expiresIn,
		"user":         newUser.ToResponse(),
	}
	log.Printf("📤 [RESP-201] Envoi de la réponse de création: UserID: %d, ExpiresIn: %d", newUser.ID, expiresIn)
	json.NewEncoder(w).Encode(responsePayload)
}

// generateTokensForUser génère les tokens JWT pour un utilisateur
// ✅ MODIFIÉ: Retourne (string, string, int, error)
func generateTokensForUser(email string, userID int) (accessToken string, refreshToken string, expiresIn int, err error) {

	// ✅ LOG AJOUTÉ: Indique le début de la génération des tokens
	log.Printf("🔑 [JWT] Génération de tokens pour Email: %s, UserID: %d", email, userID)

	// Générer le token d'accès (expire dans 24 heures)
	// ✅ MODIFIÉ: Calcul de l'expiration en secondes pour 'expiresIn'
	expirationDuration := 24 * time.Hour
	expirationTime := time.Now().Add(expirationDuration)
	expiresIn = int(expirationDuration.Seconds()) // 86400

	claims := &claims{
		Email: email,
		ID:    userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	accessToken, err = token.SignedString(jwtKey)
	if err != nil {
		return "", "", 0, fmt.Errorf("erreur lors de la génération du token d'accès: %w", err)
	}

	// Générer le refresh token (expire dans 7 jours)
	refreshExpirationTime := time.Now().Add(7 * 24 * time.Hour)
	refreshClaims := &refreshClaims{
		Email: email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(refreshExpirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	refreshTokenObj := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshToken, err = refreshTokenObj.SignedString(jwtKey)
	if err != nil {
		return "", "", 0, fmt.Errorf("erreur lors de la génération du refresh token: %w", err)
	}

	// ✅ LOG AJOUTÉ: Confirme la génération et la durée
	log.Printf("✅ [JWT] Tokens générés. AccessToken Expiration: %s, expiresIn: %d secondes", expirationDuration, expiresIn)

	return accessToken, refreshToken, expiresIn, nil
}
