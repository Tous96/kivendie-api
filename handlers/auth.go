package handlers

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"kivendi-backend/config"
	"kivendi-backend/models"
	"kivendi-backend/services"
)

// Déclaration de la clé JWT à l'échelle du package, initialisée depuis main.go.
var jwtKey []byte

// SetJWTKey est utilisé pour définir la clé JWT au démarrage de l'application.
func SetJWTKey(key string) {
	jwtKey = []byte(key)
}

// Struct pour la requête de connexion
type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Claims pour le token JWT
type claims struct {
	Email string `json:"email"`
	ID    int    `json:"id"`
	jwt.RegisteredClaims
}

// Claims pour le token de rafraîchissement
type refreshClaims struct {
	Email string `json:"email"`
	jwt.RegisteredClaims
}

// Clé de contexte pour l'ID de l'utilisateur.
type contextKey string

const userIDContextKey contextKey = "userID"

// generateVerificationCode génère un code de vérification à 4 chiffres.
func generateVerificationCode() (string, error) {
	bytes := make([]byte, 2)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	// Convertit les 2 octets en un entier 16 bits non signé pour un nombre plus grand
	code := int(bytes[0])<<8 | int(bytes[1])
	// S'assure que le code est toujours à 4 chiffres
	return fmt.Sprintf("%04d", code%10000), nil
}

// RegisterHandler gère l'inscription des nouveaux utilisateurs.
// RegisterHandler gère l'inscription des nouveaux utilisateurs
func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Début du traitement de l'inscription...")

	// Utiliser la structure RegisterRequest définie dans models
	var req models.RegisterRequest

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		log.Printf("Erreur lors du décodage de la requête: %v", err)
		http.Error(w, "Données de la requête invalides", http.StatusBadRequest)
		return
	}

	log.Printf("Données reçues: Email=%s, FirstName=%s, LastName=%s, AccountType=%s",
		req.Email, req.FirstName, req.LastName, req.AccountType)

	// Validation des champs obligatoires
	if req.Email == "" || req.Password == "" || req.FirstName == "" ||
		req.LastName == "" || req.AccountType == "" {
		log.Println("Champs obligatoires manquants")
		http.Error(w, "Veuillez remplir tous les champs obligatoires", http.StatusBadRequest)
		return
	}

	// Vérifier si l'email existe déjà
	var existingID int
	err = config.DB.QueryRow("SELECT id FROM users WHERE email = $1", req.Email).Scan(&existingID)
	if err == nil {
		log.Printf("Email %s déjà utilisé", req.Email)
		http.Error(w, "Cet email est déjà utilisé", http.StatusConflict)
		return
	} else if err != sql.ErrNoRows {
		log.Printf("Erreur lors de la vérification de l'email: %v", err)
		http.Error(w, "Erreur de base de données", http.StatusInternalServerError)
		return
	}

	// Hachage du mot de passe
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Erreur lors du hachage du mot de passe: %v", err)
		http.Error(w, "Impossible de hacher le mot de passe", http.StatusInternalServerError)
		return
	}

	// Génération du code de vérification
	code, err := generateVerificationCode()
	if err != nil {
		log.Printf("Erreur lors de la génération du code: %v", err)
		http.Error(w, "Impossible de générer le code de vérification", http.StatusInternalServerError)
		return
	}
	log.Printf("Code de vérification généré pour %s: %s", req.Email, code)

	// Conversion des champs optionnels en sql.NullString
	shopName := sql.NullString{
		String: req.ShopName,
		Valid:  req.ShopName != "",
	}

	avatarURL := sql.NullString{
		String: req.AvatarURL,
		Valid:  req.AvatarURL != "",
	}

	verificationCode := sql.NullString{
		String: code,
		Valid:  true,
	}

	// Insertion dans la base de données
	query := `
		INSERT INTO users (
			first_name, last_name, email, password_hash, 
			account_type, shop_name, avatar_url, verification_code,
			is_verified, is_blocked, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id
	`

	var userID int
	err = config.DB.QueryRow(
		query,
		req.FirstName,
		req.LastName,
		req.Email,
		string(hashedPassword),
		req.AccountType,
		shopName,
		avatarURL,
		verificationCode,
		false, // is_verified
		false, // is_blocked
		time.Now(),
		time.Now(),
	).Scan(&userID)

	if err != nil {
		log.Printf("Erreur lors de l'insertion dans la base de données: %v", err)

		// Vérifier si c'est une erreur de contrainte d'unicité
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
			http.Error(w, "Cet email est déjà utilisé", http.StatusConflict)
			return
		}

		http.Error(w, "Erreur lors de l'insertion dans la base de données", http.StatusInternalServerError)
		return
	}

	log.Printf("Utilisateur créé avec succès - ID: %d, Email: %s", userID, req.Email)

	// Envoyer l'email avec le code de vérification
	err = services.SendVerificationEmail(req.Email, code)
	if err != nil {
		log.Printf("Erreur lors de l'envoi de l'email: %v", err)
		// Ne pas bloquer l'inscription même si l'email ne part pas
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Inscription réussie. Un code de vérification a été envoyé à votre email.",
		"email":   req.Email,
	})
}

// VerifyHandler gère la vérification du code utilisateur et connecte automatiquement l'utilisateur
func VerifyHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Début de la vérification du compte")

	var requestData struct {
		Email string `json:"email"`
		Code  string `json:"code"`
	}
	err := json.NewDecoder(r.Body).Decode(&requestData)
	if err != nil {
		log.Printf("Erreur de décodage: %v", err)
		http.Error(w, "Données de la requête invalides", http.StatusBadRequest)
		return
	}

	// Recherche de l'utilisateur par email
	var user models.User
	var storedCode sql.NullString
	err = config.DB.QueryRow(`
		SELECT id, first_name, last_name, email, account_type, shop_name, 
		       avatar_url, is_verified, is_blocked, verification_code, created_at, updated_at
		FROM users WHERE email = $1
	`, requestData.Email).Scan(
		&user.ID,
		&user.FirstName,
		&user.LastName,
		&user.Email,
		&user.AccountType,
		&user.ShopName,
		&user.AvatarURL,
		&user.IsVerified,
		&user.IsBlocked,
		&storedCode,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			log.Println("Utilisateur non trouvé")
			http.Error(w, "Utilisateur non trouvé", http.StatusNotFound)
			return
		}
		log.Printf("Erreur de base de données: %v", err)
		http.Error(w, "Erreur de base de données", http.StatusInternalServerError)
		return
	}

	// Vérifier si le compte est déjà vérifié
	if user.IsVerified {
		log.Printf("Compte déjà vérifié: %s", user.Email)
		http.Error(w, "Compte déjà vérifié", http.StatusConflict)
		return
	}

	// Vérifier le code de vérification
	if !storedCode.Valid || storedCode.String != requestData.Code {
		log.Printf("Code incorrect pour: %s", user.Email)
		http.Error(w, "Code de vérification incorrect", http.StatusUnauthorized)
		return
	}

	// Mettre à jour l'utilisateur comme vérifié
	_, err = config.DB.Exec(
		"UPDATE users SET is_verified = TRUE, verification_code = NULL, updated_at = $1 WHERE email = $2",
		time.Now(),
		requestData.Email,
	)
	if err != nil {
		log.Printf("Erreur de mise à jour: %v", err)
		http.Error(w, "Impossible de mettre à jour le statut de l'utilisateur", http.StatusInternalServerError)
		return
	}

	// Mettre à jour le statut de vérification dans l'objet user
	user.IsVerified = true

	// Créer les tokens JWT pour la connexion automatique
	accessTokenExpirationTime := time.Now().Add(7 * 24 * time.Hour)
	accessTokenClaims := &claims{
		Email: user.Email,
		ID:    user.ID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(accessTokenExpirationTime),
		},
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessTokenClaims)
	accessTokenString, err := accessToken.SignedString(jwtKey)
	if err != nil {
		log.Printf("Erreur lors de la création du token d'accès: %v", err)
		http.Error(w, "Impossible de créer le token d'accès", http.StatusInternalServerError)
		return
	}

	refreshTokenExpirationTime := time.Now().Add(90 * 24 * time.Hour)
	refreshTokenClaims := &refreshClaims{
		Email: user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(refreshTokenExpirationTime),
		},
	}

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshTokenClaims)
	refreshTokenString, err := refreshToken.SignedString(jwtKey)
	if err != nil {
		log.Printf("Erreur lors de la création du refresh token: %v", err)
		http.Error(w, "Impossible de créer le refresh token", http.StatusInternalServerError)
		return
	}

	log.Printf("Compte vérifié et connecté avec succès: %s", user.Email)

	// Retourner les tokens et les informations utilisateur
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":      "Compte vérifié avec succès. Vous êtes maintenant connecté.",
		"user":         user.ToResponse(),
		"token":        accessTokenString,
		"refreshToken": refreshTokenString,
		"expiresIn":    int(7 * 24 * 3600), // 7 jours en secondes
	})
}

// ResendVerificationCodeHandler gère le renvoi du code de vérification
func ResendVerificationCodeHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Début du renvoi du code de vérification")

	var requestData struct {
		Email string `json:"email"`
	}

	err := json.NewDecoder(r.Body).Decode(&requestData)
	if err != nil {
		log.Printf("Erreur de décodage: %v", err)
		http.Error(w, "Données de la requête invalides", http.StatusBadRequest)
		return
	}

	// Validation de l'email
	if requestData.Email == "" {
		log.Println("Email manquant")
		http.Error(w, "L'email est requis", http.StatusBadRequest)
		return
	}

	// Recherche de l'utilisateur par email
	var user models.User
	var isVerified bool
	err = config.DB.QueryRow(`
		SELECT id, email, is_verified, is_blocked
		FROM users WHERE email = $1
	`, requestData.Email).Scan(
		&user.ID,
		&user.Email,
		&isVerified,
		&user.IsBlocked,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("Utilisateur non trouvé: %s", requestData.Email)
			http.Error(w, "Aucun compte n'est associé à cet email", http.StatusNotFound)
			return
		}
		log.Printf("Erreur de base de données: %v", err)
		http.Error(w, "Erreur de base de données", http.StatusInternalServerError)
		return
	}

	// Vérifier si le compte est bloqué
	if user.IsBlocked {
		log.Printf("Tentative de renvoi de code pour un compte bloqué: %s", user.Email)
		http.Error(w, "Votre compte a été bloqué. Contactez l'administrateur.", http.StatusForbidden)
		return
	}

	// Vérifier si le compte est déjà vérifié
	if isVerified {
		log.Printf("Compte déjà vérifié: %s", user.Email)
		http.Error(w, "Votre compte est déjà vérifié", http.StatusConflict)
		return
	}

	// Générer un nouveau code de vérification
	code, err := generateVerificationCode()
	if err != nil {
		log.Printf("Erreur lors de la génération du code: %v", err)
		http.Error(w, "Impossible de générer le code de vérification", http.StatusInternalServerError)
		return
	}
	log.Printf("Nouveau code de vérification généré pour %s: %s", user.Email, code)

	// Mettre à jour le code de vérification dans la base de données
	verificationCode := sql.NullString{
		String: code,
		Valid:  true,
	}

	_, err = config.DB.Exec(
		"UPDATE users SET verification_code = $1, updated_at = $2 WHERE email = $3",
		verificationCode,
		time.Now(),
		requestData.Email,
	)
	if err != nil {
		log.Printf("Erreur de mise à jour: %v", err)
		http.Error(w, "Impossible de mettre à jour le code de vérification", http.StatusInternalServerError)
		return
	}
	// Envoyer l'email avec le nouveau code de vérification
	err = services.SendVerificationEmail(requestData.Email, code)
	if err != nil {
		log.Printf("Erreur lors de l'envoi de l'email: %v", err)
	}

	log.Printf("Code de vérification renvoyé avec succès pour: %s", requestData.Email)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Un nouveau code de vérification a été envoyé à votre email.",
		"email":   requestData.Email,
	})
}

// LoginHandler gère la connexion des utilisateurs
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Début du traitement de la requête de connexion.")

	var creds credentials
	err := json.NewDecoder(r.Body).Decode(&creds)
	if err != nil {
		log.Printf("Erreur lors du décodage de la requête: %v", err)
		http.Error(w, "Données de la requête invalides", http.StatusBadRequest)
		return
	}

	var user models.User
	err = config.DB.QueryRow(`
		SELECT id, first_name, last_name, email, account_type, shop_name, 
		       avatar_url, is_verified, is_blocked, password_hash, created_at, updated_at
		FROM users WHERE email = $1
	`, creds.Email).Scan(
		&user.ID,
		&user.FirstName,
		&user.LastName,
		&user.Email,
		&user.AccountType,
		&user.ShopName,
		&user.AvatarURL,
		&user.IsVerified,
		&user.IsBlocked,
		&user.PasswordHash,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			log.Println("Utilisateur non trouvé")
			http.Error(w, "Email ou mot de passe incorrect", http.StatusUnauthorized)
			return
		}
		log.Printf("Erreur de base de données: %v", err)
		http.Error(w, "Erreur de base de données", http.StatusInternalServerError)
		return
	}

	// Vérifier si le compte est bloqué
	if user.IsBlocked {
		log.Printf("Tentative de connexion d'un compte bloqué: %s", user.Email)
		http.Error(w, "Votre compte a été bloqué. Contactez l'administrateur.", http.StatusForbidden)
		return
	}

	// Vérifier si le compte est vérifié
	if !user.IsVerified {
		log.Println("Compte non vérifié")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":                 "account_not_verified",
			"message":               "Votre compte n'a pas été vérifié. Veuillez vérifier votre email.",
			"email":                 user.Email,
			"requires_verification": true,
		})
		return
	}

	// Vérifier le mot de passe
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(creds.Password))
	if err != nil {
		log.Printf("Mot de passe incorrect pour: %s", user.Email)
		http.Error(w, "Email ou mot de passe incorrect", http.StatusUnauthorized)
		return
	}

	// Créer les tokens
	accessTokenExpirationTime := time.Now().Add(7 * 24 * time.Hour)
	accessTokenClaims := &claims{
		Email: user.Email,
		ID:    user.ID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(accessTokenExpirationTime),
		},
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessTokenClaims)
	accessTokenString, err := accessToken.SignedString(jwtKey)
	if err != nil {
		log.Printf("Erreur lors de la création du token d'accès: %v", err)
		http.Error(w, "Impossible de créer le token d'accès", http.StatusInternalServerError)
		return
	}

	refreshTokenExpirationTime := time.Now().Add(90 * 24 * time.Hour)
	refreshTokenClaims := &refreshClaims{
		Email: user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(refreshTokenExpirationTime),
		},
	}

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshTokenClaims)
	refreshTokenString, err := refreshToken.SignedString(jwtKey)
	if err != nil {
		log.Printf("Erreur lors de la création du refresh token: %v", err)
		http.Error(w, "Impossible de créer le refresh token", http.StatusInternalServerError)
		return
	}

	log.Printf("Connexion réussie pour: %s", user.Email)

	// Retourner la réponse avec expiresIn
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"user":         user.ToResponse(),
		"token":        accessTokenString,
		"refreshToken": refreshTokenString,
		"expiresIn":    int(7 * 24 * 3600), // 7 jours en secondes
	})
}

// RefreshHandler gère le rafraîchissement des tokens
func RefreshHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Début du rafraîchissement de token")

	var requestBody struct {
		RefreshToken string `json:"refreshToken"`
	}

	err := json.NewDecoder(r.Body).Decode(&requestBody)
	if err != nil {
		log.Printf("Erreur de décodage: %v", err)
		http.Error(w, "Données invalides", http.StatusBadRequest)
		return
	}

	// Valider le refresh token
	token, err := jwt.ParseWithClaims(requestBody.RefreshToken, &refreshClaims{}, func(token *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	})

	if err != nil || !token.Valid {
		log.Printf("Refresh token invalide: %v", err)
		http.Error(w, "Token de rafraîchissement invalide", http.StatusUnauthorized)
		return
	}

	refreshTokenData, ok := token.Claims.(*refreshClaims)
	if !ok {
		log.Println("Claims invalides")
		http.Error(w, "Token invalide", http.StatusUnauthorized)
		return
	}

	// Récupérer l'utilisateur
	var user models.User
	err = config.DB.QueryRow(`
		SELECT id, first_name, last_name, email, account_type, shop_name,
		       avatar_url, is_verified, is_blocked, created_at, updated_at
		FROM users WHERE email = $1
	`, refreshTokenData.Email).Scan(
		&user.ID,
		&user.FirstName,
		&user.LastName,
		&user.Email,
		&user.AccountType,
		&user.ShopName,
		&user.AvatarURL,
		&user.IsVerified,
		&user.IsBlocked,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			log.Println("Utilisateur non trouvé")
			http.Error(w, "Utilisateur non trouvé", http.StatusNotFound)
			return
		}
		log.Printf("Erreur de BDD: %v", err)
		http.Error(w, "Erreur de base de données", http.StatusInternalServerError)
		return
	}

	// Vérifier que le compte n'est pas bloqué
	if user.IsBlocked {
		log.Printf("Compte bloqué: %s", user.Email)
		http.Error(w, "Compte bloqué", http.StatusForbidden)
		return
	}

	// Créer de nouveaux tokens
	accessTokenExpirationTime := time.Now().Add(7 * 24 * time.Hour)
	accessTokenClaims := &claims{
		Email: user.Email,
		ID:    user.ID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(accessTokenExpirationTime),
		},
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessTokenClaims)
	accessTokenString, err := accessToken.SignedString(jwtKey)
	if err != nil {
		log.Printf("Erreur création access token: %v", err)
		http.Error(w, "Erreur lors de la création du token", http.StatusInternalServerError)
		return
	}

	refreshTokenExpirationTime := time.Now().Add(90 * 24 * time.Hour)
	newRefreshTokenClaims := &refreshClaims{
		Email: user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(refreshTokenExpirationTime),
		},
	}

	newRefreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, newRefreshTokenClaims)
	newRefreshTokenString, err := newRefreshToken.SignedString(jwtKey)
	if err != nil {
		log.Printf("Erreur création refresh token: %v", err)
		http.Error(w, "Erreur lors de la création du refresh token", http.StatusInternalServerError)
		return
	}

	log.Printf("Tokens rafraîchis pour: %s", user.Email)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"user":         user.ToResponse(),
		"token":        accessTokenString,
		"refreshToken": newRefreshTokenString,
		"expiresIn":    int(7 * 24 * 3600),
	})
}

// ValidateToken est un middleware qui valide le token JWT.
func ValidateToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Tente de récupérer le jeton de l'en-tête 'Authorization'
		tokenString := r.Header.Get("Authorization")

		// Si l'en-tête d'autorisation est vide, vérifiez les paramètres de l'URL (pour les WebSockets)
		if tokenString == "" {
			tokenString = r.URL.Query().Get("token")
			// Si le jeton est trouvé dans les paramètres de requête,
			// il est déjà le jeton pur, pas "Bearer <token>", donc pas besoin de le diviser.
			if tokenString != "" {
				// S'il est là, on peut passer directement à la validation.
				goto validateToken
			}
		} else {
			// Le token est au format "Bearer <token>"
			parts := strings.Split(tokenString, " ")
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				http.Error(w, "Format d'en-tête d'autorisation invalide", http.StatusUnauthorized)
				return
			}
			tokenString = parts[1]
		}

	validateToken: // Étiquette pour sauter directement ici si le jeton vient des paramètres de requête

		// Si le jeton est toujours vide après les deux vérifications, il y a une erreur.
		if tokenString == "" {
			http.Error(w, "Jeton manquant", http.StatusUnauthorized)
			return
		}

		// Le reste de la logique de validation du jeton
		token, err := jwt.ParseWithClaims(tokenString, &claims{}, func(token *jwt.Token) (interface{}, error) {
			return jwtKey, nil
		})

		if err != nil {
			if strings.Contains(err.Error(), "token is expired") {
				http.Error(w, "Token expiré", http.StatusUnauthorized)
				return
			}
			http.Error(w, "Token invalide: "+err.Error(), http.StatusUnauthorized)
			return
		}

		if !token.Valid {
			http.Error(w, "Token invalide", http.StatusUnauthorized)
			return
		}

		// Extraire les revendications et l'ID utilisateur
		claims, ok := token.Claims.(*claims)
		if !ok {
			http.Error(w, "Revendications de token invalides", http.StatusUnauthorized)
			return
		}

		// Ajouter l'ID de l'utilisateur au contexte de la requête
		ctx := context.WithValue(r.Context(), userIDContextKey, claims.ID)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}

// UpdateProfileHandler gère la modification du profil utilisateur
func UpdateProfileHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Début de la mise à jour du profil")

	// Récupérer l'ID utilisateur du contexte (fourni par le middleware JWT)
	userID, ok := r.Context().Value(userIDContextKey).(int)
	if !ok {
		log.Println("ID utilisateur non trouvé dans le contexte")
		http.Error(w, "Non autorisé", http.StatusUnauthorized)
		return
	}

	var updateData struct {
		FirstName    *string `json:"first_name,omitempty"`
		LastName     *string `json:"last_name,omitempty"`
		Email        *string `json:"email,omitempty"`
		Password     *string `json:"password,omitempty"`
		AccountType  *string `json:"account_type,omitempty"`
		ShopName     *string `json:"shop_name,omitempty"`
		AvatarURL    *string `json:"avatar_url,omitempty"`
		AvatarBase64 *string `json:"avatar_base64,omitempty"`
	}

	err := json.NewDecoder(r.Body).Decode(&updateData)
	if err != nil {
		log.Printf("Erreur de décodage: %v", err)
		http.Error(w, "Données invalides", http.StatusBadRequest)
		return
	}

	// Construction dynamique de la requête SQL
	var setParts []string
	var args []interface{}
	argIndex := 1

	if updateData.FirstName != nil {
		setParts = append(setParts, fmt.Sprintf("first_name = $%d", argIndex))
		args = append(args, *updateData.FirstName)
		argIndex++
	}

	if updateData.LastName != nil {
		setParts = append(setParts, fmt.Sprintf("last_name = $%d", argIndex))
		args = append(args, *updateData.LastName)
		argIndex++
	}

	if updateData.Email != nil {
		// Vérifier l'unicité de l'email
		var existingUserID int
		err = config.DB.QueryRow(
			"SELECT id FROM users WHERE email = $1 AND id != $2",
			*updateData.Email, userID,
		).Scan(&existingUserID)

		if err == nil {
			log.Printf("Email déjà utilisé par l'utilisateur %d", existingUserID)
			http.Error(w, "Cet email est déjà utilisé", http.StatusConflict)
			return
		} else if err != sql.ErrNoRows {
			log.Printf("Erreur lors de la vérification de l'email: %v", err)
			http.Error(w, "Erreur de base de données", http.StatusInternalServerError)
			return
		}

		setParts = append(setParts, fmt.Sprintf("email = $%d", argIndex))
		args = append(args, *updateData.Email)
		argIndex++
	}

	if updateData.Password != nil {
		hashedPassword, err := bcrypt.GenerateFromPassword(
			[]byte(*updateData.Password),
			bcrypt.DefaultCost,
		)
		if err != nil {
			log.Printf("Erreur de hachage: %v", err)
			http.Error(w, "Erreur lors du hachage du mot de passe", http.StatusInternalServerError)
			return
		}
		setParts = append(setParts, fmt.Sprintf("password_hash = $%d", argIndex))
		args = append(args, string(hashedPassword))
		argIndex++
	}

	if updateData.AccountType != nil {
		setParts = append(setParts, fmt.Sprintf("account_type = $%d", argIndex))
		args = append(args, *updateData.AccountType)
		argIndex++
	}

	if updateData.ShopName != nil {
		shopName := sql.NullString{
			String: *updateData.ShopName,
			Valid:  *updateData.ShopName != "",
		}
		setParts = append(setParts, fmt.Sprintf("shop_name = $%d", argIndex))
		args = append(args, shopName)
		argIndex++
	}

	// Gestion de l'avatar
	if updateData.AvatarBase64 != nil && *updateData.AvatarBase64 != "" {
		// Upload vers S3
		awsService, err := services.NewAWSService()
		if err != nil {
			log.Printf("Erreur AWS Service: %v", err)
			http.Error(w, "Erreur lors de l'initialisation du service de stockage", http.StatusInternalServerError)
			return
		}

		newAvatarURL, err := awsService.UploadAvatar(*updateData.AvatarBase64)
		if err != nil {
			log.Printf("Erreur upload avatar: %v", err)
			http.Error(w, "Erreur lors du téléchargement de l'avatar", http.StatusInternalServerError)
			return
		}

		avatarURL := sql.NullString{
			String: newAvatarURL,
			Valid:  true,
		}
		setParts = append(setParts, fmt.Sprintf("avatar_url = $%d", argIndex))
		args = append(args, avatarURL)
		argIndex++
	} else if updateData.AvatarURL != nil {
		avatarURL := sql.NullString{
			String: *updateData.AvatarURL,
			Valid:  *updateData.AvatarURL != "",
		}
		setParts = append(setParts, fmt.Sprintf("avatar_url = $%d", argIndex))
		args = append(args, avatarURL)
		argIndex++
	}

	// Vérifier qu'il y a au moins un champ à mettre à jour
	if len(setParts) == 0 {
		log.Println("Aucun champ à mettre à jour")
		http.Error(w, "Aucun champ à mettre à jour", http.StatusBadRequest)
		return
	}

	// Ajouter updated_at
	setParts = append(setParts, fmt.Sprintf("updated_at = $%d", argIndex))
	args = append(args, time.Now())
	argIndex++

	// Ajouter l'ID utilisateur pour WHERE
	args = append(args, userID)

	// Construire et exécuter la requête
	query := fmt.Sprintf(
		"UPDATE users SET %s WHERE id = $%d",
		strings.Join(setParts, ", "),
		argIndex,
	)

	log.Printf("Exécution de la requête: %s", query)

	result, err := config.DB.Exec(query, args...)
	if err != nil {
		log.Printf("Erreur lors de la mise à jour: %v", err)
		http.Error(w, "Erreur lors de la mise à jour", http.StatusInternalServerError)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("Erreur RowsAffected: %v", err)
		http.Error(w, "Erreur de base de données", http.StatusInternalServerError)
		return
	}

	if rowsAffected == 0 {
		log.Printf("Aucune ligne mise à jour pour l'utilisateur %d", userID)
		http.Error(w, "Utilisateur non trouvé", http.StatusNotFound)
		return
	}

	// Récupérer les données mises à jour
	var user models.User
	err = config.DB.QueryRow(`
		SELECT id, first_name, last_name, email, account_type, shop_name,
		       avatar_url, is_verified, is_blocked, created_at, updated_at
		FROM users WHERE id = $1
	`, userID).Scan(
		&user.ID,
		&user.FirstName,
		&user.LastName,
		&user.Email,
		&user.AccountType,
		&user.ShopName,
		&user.AvatarURL,
		&user.IsVerified,
		&user.IsBlocked,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		log.Printf("Erreur lors de la récupération: %v", err)
		http.Error(w, "Erreur lors de la récupération des données", http.StatusInternalServerError)
		return
	}

	log.Printf("Profil mis à jour avec succès pour l'utilisateur %d", userID)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Profil mis à jour avec succès",
		"user":    user.ToResponse(),
	})
}

// ProfileHandler est un exemple de route protégée par JWT.
func ProfileHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Bienvenue sur votre profil ! Ce contenu est protégé."})
}
