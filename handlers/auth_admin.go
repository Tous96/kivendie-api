package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"kivendi-backend/config"
	"kivendi-backend/services"
)

// AdminCredentials pour la requête de connexion admin
type adminCredentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// AdminClaims pour le token JWT admin
type adminClaims struct {
	Email string `json:"email"`
	ID    int    `json:"id"`
	Role  string `json:"role"` // "admin" ou "moderateur"
	jwt.RegisteredClaims
}

// AdminRefreshClaims pour le token de rafraîchissement admin
type adminRefreshClaims struct {
	Email string `json:"email"`
	Role  string `json:"role"`
	jwt.RegisteredClaims
}

// Clés de contexte pour l'admin
type adminContextKey string

const (
	adminIDContextKey   adminContextKey = "adminID"
	adminRoleContextKey adminContextKey = "adminRole"
)

// AdminLoginHandler gère la connexion des administrateurs
func AdminLoginHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Début du traitement de la requête de connexion admin.")
	var creds adminCredentials
	err := json.NewDecoder(r.Body).Decode(&creds)
	if err != nil {
		log.Printf("Erreur lors du décodage de la requête: %v", err)
		http.Error(w, "Données de la requête invalides", http.StatusBadRequest)
		return
	}
	log.Println("Données de la requête décodées avec succès.")

	var admin struct {
		ID           int
		Email        string
		PasswordHash string
		Role         string
		FirstName    string
		LastName     string
		IsActive     bool
		// MODIFICATION: Ajout du champ pour l'URL de l'avatar.
		// sql.NullString est utilisé pour gérer correctement les valeurs NULL de la base de données.
		AvatarURL sql.NullString
	}

	log.Printf("Recherche de l'admin avec l'email: %s", creds.Email)
	// MODIFICATION: Ajout de 'avatar_url' à la requête SQL.
	err = config.DB.QueryRow(`
		SELECT id, email, password_hash, role, first_name, last_name, is_active, avatar_url 
		FROM admins 
		WHERE email = $1`,
		creds.Email,
	).Scan(
		&admin.ID,
		&admin.Email,
		&admin.PasswordHash,
		&admin.Role,
		&admin.FirstName,
		&admin.LastName,
		&admin.IsActive,
		// MODIFICATION: Récupération de la nouvelle colonne.
		&admin.AvatarURL,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			log.Println("Admin non trouvé dans la base de données.")
			http.Error(w, "Email ou mot de passe incorrect", http.StatusUnauthorized)
			return
		}
		log.Printf("Erreur de base de données lors de la recherche de l'admin: %v", err)
		http.Error(w, "Erreur de base de données", http.StatusInternalServerError)
		return
	}
	log.Println("Admin trouvé avec succès.")

	// Vérifier si le compte est actif
	if !admin.IsActive {
		log.Println("Compte admin désactivé. Connexion refusée.")
		http.Error(w, "Votre compte a été désactivé. Contactez un super administrateur.", http.StatusForbidden)
		return
	}
	log.Println("Compte admin actif. Comparaison des mots de passe.")

	// Comparer le mot de passe haché
	err = bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(creds.Password))
	if err != nil {
		log.Printf("Mot de passe incorrect pour l'admin: %s", admin.Email)
		http.Error(w, "Email ou mot de passe incorrect", http.StatusUnauthorized)
		return
	}
	log.Println("Mots de passe correspondent. Création des tokens.")

	// Mettre à jour la dernière connexion
	_, err = config.DB.Exec(`
		UPDATE admins 
		SET last_login = $1 
		WHERE id = $2`,
		time.Now(),
		admin.ID,
	)
	if err != nil {
		log.Printf("Erreur lors de la mise à jour de last_login: %v", err)
		// Continue malgré l'erreur
	}

	// Créer le token d'accès admin
	accessTokenExpirationTime := time.Now().Add(12 * time.Hour)
	accessTokenClaims := &adminClaims{
		Email: admin.Email,
		ID:    admin.ID,
		Role:  admin.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(accessTokenExpirationTime),
		},
	}
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessTokenClaims)
	accessTokenString, err := accessToken.SignedString(jwtKey)
	if err != nil {
		log.Printf("Impossible de créer le token d'accès admin: %v", err)
		http.Error(w, "Impossible de créer le token d'accès", http.StatusInternalServerError)
		return
	}

	// Créer le token de rafraîchissement admin
	refreshTokenExpirationTime := time.Now().Add(7 * 24 * time.Hour)
	refreshTokenClaims := &adminRefreshClaims{
		Email: admin.Email,
		Role:  admin.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(refreshTokenExpirationTime),
		},
	}
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshTokenClaims)
	refreshTokenString, err := refreshToken.SignedString(jwtKey)
	if err != nil {
		log.Printf("Impossible de créer le token de rafraîchissement admin: %v", err)
		http.Error(w, "Impossible de créer le token de rafraîchissement", http.StatusInternalServerError)
		return
	}
	log.Println("Tokens d'accès et de rafraîchissement admin créés avec succès.")

	// Renvoyer l'objet admin complet et les tokens
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"admin": map[string]interface{}{
			"id":        admin.ID,
			"firstName": admin.FirstName,
			"lastName":  admin.LastName,
			"email":     admin.Email,
			"role":      admin.Role,
			"isActive":  admin.IsActive,
			// MODIFICATION: Ajout de l'URL de l'avatar à la réponse JSON.
			// .String convertit la valeur en chaîne de caractères (vide si NULL).
			// Le nom est en camelCase pour être cohérent avec le front-end.
			"avatarUrl": admin.AvatarURL.String,
		},
		"token":        accessTokenString,
		"refreshToken": refreshTokenString,
	})
	log.Println("Réponse de connexion admin envoyée avec succès.")
}

// AdminRefreshHandler gère le rafraîchissement des tokens admin
func AdminRefreshHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Début du traitement de la requête de rafraîchissement de jeton admin.")
	var requestBody struct {
		RefreshToken string `json:"refreshToken"`
	}
	err := json.NewDecoder(r.Body).Decode(&requestBody)
	if err != nil {
		log.Printf("Erreur lors du décodage de la requête de rafraîchissement: %v", err)
		http.Error(w, "Données de la requête invalides", http.StatusBadRequest)
		return
	}

	// Analyser le token de rafraîchissement
	log.Println("Analyse du token de rafraîchissement admin...")
	token, err := jwt.ParseWithClaims(requestBody.RefreshToken, &adminRefreshClaims{}, func(token *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	})

	if err != nil || !token.Valid {
		log.Printf("Token de rafraîchissement admin invalide ou expiré: %v", err)
		http.Error(w, "Token de rafraîchissement invalide", http.StatusUnauthorized)
		return
	}

	refreshTokenData, ok := token.Claims.(*adminRefreshClaims)
	if !ok {
		log.Println("Échec de l'extraction des revendications du token de rafraîchissement admin.")
		http.Error(w, "Token de rafraîchissement invalide", http.StatusUnauthorized)
		return
	}
	log.Printf("Token de rafraîchissement admin analysé. Email: %s", refreshTokenData.Email)

	// Récupérer les données de l'admin depuis la base de données
	log.Println("Récupération des données admin à partir de la base de données...")
	var admin struct {
		ID        int
		Email     string
		Role      string
		FirstName string
		LastName  string
		IsActive  bool
		// MODIFICATION: Ajout du champ pour l'URL de l'avatar.
		AvatarURL sql.NullString
	}
	// MODIFICATION: Ajout de 'avatar_url' à la requête SQL.
	err = config.DB.QueryRow(`
		SELECT id, email, role, first_name, last_name, is_active, avatar_url 
		FROM admins 
		WHERE email = $1`,
		refreshTokenData.Email,
	).Scan(
		&admin.ID,
		&admin.Email,
		&admin.Role,
		&admin.FirstName,
		&admin.LastName,
		&admin.IsActive,
		// MODIFICATION: Récupération de la nouvelle colonne.
		&admin.AvatarURL,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("Admin avec l'email %s non trouvé lors du rafraîchissement.", refreshTokenData.Email)
			http.Error(w, "Admin non trouvé", http.StatusNotFound)
			return
		}
		log.Printf("Erreur de base de données lors de la récupération des données admin: %v", err)
		http.Error(w, "Erreur de base de données", http.StatusInternalServerError)
		return
	}

	if !admin.IsActive {
		log.Println("Compte admin désactivé lors du rafraîchissement.")
		http.Error(w, "Votre compte a été désactivé", http.StatusForbidden)
		return
	}

	log.Printf("Données admin récupérées avec succès pour l'email: %s", admin.Email)

	// Créer un nouveau token d'accès
	log.Println("Création d'un nouveau token d'accès admin...")
	accessTokenExpirationTime := time.Now().Add(12 * time.Hour)
	accessTokenClaims := &adminClaims{
		Email: admin.Email,
		ID:    admin.ID,
		Role:  admin.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(accessTokenExpirationTime),
		},
	}
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessTokenClaims)
	accessTokenString, err := accessToken.SignedString(jwtKey)
	if err != nil {
		log.Printf("Impossible de créer un nouveau token d'accès admin: %v", err)
		http.Error(w, "Impossible de créer un nouveau token d'accès", http.StatusInternalServerError)
		return
	}

	// Créer un nouveau token de rafraîchissement
	log.Println("Création d'un nouveau token de rafraîchissement admin...")
	refreshTokenExpirationTime := time.Now().Add(7 * 24 * time.Hour)
	newRefreshTokenClaims := &adminRefreshClaims{
		Email: admin.Email,
		Role:  admin.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(refreshTokenExpirationTime),
		},
	}
	newRefreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, newRefreshTokenClaims)
	newRefreshTokenString, err := newRefreshToken.SignedString(jwtKey)
	if err != nil {
		log.Printf("Impossible de créer un nouveau token de rafraîchissement admin: %v", err)
		http.Error(w, "Impossible de créer un nouveau token de rafraîchissement", http.StatusInternalServerError)
		return
	}

	// Renvoyer les nouvelles données
	log.Println("Envoi de la réponse de rafraîchissement admin au client.")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"admin": map[string]interface{}{
			"id":        admin.ID,
			"firstName": admin.FirstName,
			"lastName":  admin.LastName,
			"email":     admin.Email,
			"role":      admin.Role,
			"isActive":  admin.IsActive,
			// MODIFICATION: Ajout de l'URL de l'avatar à la réponse JSON.
			"avatarUrl": admin.AvatarURL.String,
		},
		"token":        accessTokenString,
		"refreshToken": newRefreshTokenString,
	})
	log.Println("Requête de rafraîchissement admin terminée avec succès.")
}

// ValidateAdminToken est un middleware qui valide le token JWT admin
func ValidateAdminToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Récupérer le token de l'en-tête Authorization
		tokenString := r.Header.Get("Authorization")

		if tokenString == "" {
			tokenString = r.URL.Query().Get("token")
			if tokenString != "" {
				goto validateToken
			}
		} else {
			parts := strings.Split(tokenString, " ")
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				http.Error(w, "Format d'en-tête d'autorisation invalide", http.StatusUnauthorized)
				return
			}
			tokenString = parts[1]
		}

	validateToken:
		if tokenString == "" {
			http.Error(w, "Token manquant", http.StatusUnauthorized)
			return
		}

		// Parser le token
		token, err := jwt.ParseWithClaims(tokenString, &adminClaims{}, func(token *jwt.Token) (interface{}, error) {
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

		// Extraire les claims
		claims, ok := token.Claims.(*adminClaims)
		if !ok {
			http.Error(w, "Revendications de token invalides", http.StatusUnauthorized)
			return
		}

		// Vérifier que le compte est toujours actif
		var isActive bool
		err = config.DB.QueryRow("SELECT is_active FROM admins WHERE id = $1", claims.ID).Scan(&isActive)
		if err != nil {
			http.Error(w, "Erreur de vérification du compte", http.StatusInternalServerError)
			return
		}

		if !isActive {
			http.Error(w, "Compte désactivé", http.StatusForbidden)
			return
		}

		// Ajouter l'ID et le rôle de l'admin au contexte
		ctx := context.WithValue(r.Context(), adminIDContextKey, claims.ID)
		ctx = context.WithValue(ctx, adminRoleContextKey, claims.Role)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}

// RequireAdminRole middleware pour vérifier que l'utilisateur a le rôle admin
func RequireAdminRole(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role, ok := r.Context().Value(adminRoleContextKey).(string)
		if !ok || role != "admin" {
			http.Error(w, "Accès refusé. Rôle admin requis.", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireModeratorOrAdmin middleware pour vérifier que l'utilisateur a le rôle modérateur ou admin
func RequireModeratorOrAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role, ok := r.Context().Value(adminRoleContextKey).(string)
		if !ok || (role != "admin" && role != "moderateur") {
			http.Error(w, "Accès refusé. Rôle modérateur ou admin requis.", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// GetAdminProfileHandler retourne le profil de l'admin connecté
// GetAdminProfileHandler retourne le profil de l'admin connecté
func GetAdminProfileHandler(w http.ResponseWriter, r *http.Request) {
	adminID, ok := r.Context().Value(adminIDContextKey).(int)
	if !ok {
		http.Error(w, "ID admin non trouvé", http.StatusUnauthorized)
		return
	}

	var admin struct {
		ID        int       `json:"id"`
		Email     string    `json:"email"`
		Role      string    `json:"role"`
		FirstName string    `json:"first_name"`
		LastName  string    `json:"last_name"`
		IsActive  bool      `json:"is_active"`
		AvatarUrl *string   `json:"avatar_url"` // ✅ AJOUTÉ (pointeur car peut être NULL)
		CreatedAt time.Time `json:"created_at"`
		LastLogin time.Time `json:"last_login"`
	}

	err := config.DB.QueryRow(`
		SELECT id, email, role, first_name, last_name, is_active, avatar_url, created_at, last_login
		FROM admins 
		WHERE id = $1`,
		adminID,
	).Scan(
		&admin.ID,
		&admin.Email,
		&admin.Role,
		&admin.FirstName,
		&admin.LastName,
		&admin.IsActive,
		&admin.AvatarUrl, // ✅ AJOUTÉ
		&admin.CreatedAt,
		&admin.LastLogin,
	)

	if err != nil {
		log.Printf("Erreur lors de la récupération du profil admin: %v", err)
		http.Error(w, "Erreur lors de la récupération du profil", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(admin)
}

// UpdateAdminPasswordRequest structure pour la mise à jour du mot de passe
type UpdateAdminPasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

// UpdateAdminAvatarRequest structure pour la mise à jour de l'avatar
type UpdateAdminAvatarRequest struct {
	AvatarBase64 string `json:"avatarBase64"`
}

// UpdateAdminPasswordHandler permet à l'admin de modifier son mot de passe
func UpdateAdminPasswordHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Début de la mise à jour du mot de passe admin")

	// Récupérer l'ID de l'admin depuis le contexte
	adminID, ok := r.Context().Value(adminIDContextKey).(int)
	if !ok {
		log.Println("ID admin non trouvé dans le contexte")
		http.Error(w, "Non autorisé", http.StatusUnauthorized)
		return
	}

	// Décoder la requête
	var req UpdateAdminPasswordRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		log.Printf("Erreur lors du décodage de la requête: %v", err)
		http.Error(w, "Données de requête invalides", http.StatusBadRequest)
		return
	}

	// Valider les données
	if req.CurrentPassword == "" || req.NewPassword == "" {
		http.Error(w, "Tous les champs sont requis", http.StatusBadRequest)
		return
	}

	if len(req.NewPassword) < 8 {
		http.Error(w, "Le nouveau mot de passe doit contenir au moins 8 caractères", http.StatusBadRequest)
		return
	}

	// Récupérer le hash du mot de passe actuel
	var currentPasswordHash string
	err = config.DB.QueryRow(`
		SELECT password_hash 
		FROM admins 
		WHERE id = $1`,
		adminID,
	).Scan(&currentPasswordHash)

	if err != nil {
		log.Printf("Erreur lors de la récupération du mot de passe: %v", err)
		http.Error(w, "Erreur de base de données", http.StatusInternalServerError)
		return
	}

	// Vérifier le mot de passe actuel
	err = bcrypt.CompareHashAndPassword([]byte(currentPasswordHash), []byte(req.CurrentPassword))
	if err != nil {
		log.Println("Mot de passe actuel incorrect")
		http.Error(w, "Mot de passe actuel incorrect", http.StatusUnauthorized)
		return
	}

	// Hasher le nouveau mot de passe
	newPasswordHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Erreur lors du hashage du nouveau mot de passe: %v", err)
		http.Error(w, "Erreur lors du traitement du mot de passe", http.StatusInternalServerError)
		return
	}

	// Mettre à jour le mot de passe dans la base de données
	_, err = config.DB.Exec(`
		UPDATE admins 
		SET password_hash = $1 
		WHERE id = $2`,
		string(newPasswordHash),
		adminID,
	)

	if err != nil {
		log.Printf("Erreur lors de la mise à jour du mot de passe: %v", err)
		http.Error(w, "Erreur lors de la mise à jour du mot de passe", http.StatusInternalServerError)
		return
	}

	log.Printf("Mot de passe mis à jour avec succès pour l'admin ID: %d", adminID)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Mot de passe mis à jour avec succès",
	})
}

// UpdateAdminAvatarHandler permet à l'admin de modifier son avatar
func UpdateAdminAvatarHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Début de la mise à jour de l'avatar admin")

	// Récupérer l'ID de l'admin depuis le contexte
	adminID, ok := r.Context().Value(adminIDContextKey).(int)
	if !ok {
		log.Println("ID admin non trouvé dans le contexte")
		http.Error(w, "Non autorisé", http.StatusUnauthorized)
		return
	}

	// Décoder la requête
	var req UpdateAdminAvatarRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		log.Printf("Erreur lors du décodage de la requête: %v", err)
		http.Error(w, "Données de requête invalides", http.StatusBadRequest)
		return
	}

	// Valider les données
	if req.AvatarBase64 == "" {
		http.Error(w, "Image avatar requise", http.StatusBadRequest)
		return
	}

	// Initialiser le service AWS
	awsService, err := services.NewAWSService()
	if err != nil {
		log.Printf("Erreur lors de l'initialisation du service AWS: %v", err)
		http.Error(w, "Erreur de configuration du service de stockage", http.StatusInternalServerError)
		return
	}

	// Récupérer l'ancien avatar pour le supprimer (si existe)
	var oldAvatarURL sql.NullString
	err = config.DB.QueryRow(`
		SELECT avatar_url 
		FROM admins 
		WHERE id = $1`,
		adminID,
	).Scan(&oldAvatarURL)

	if err != nil && err != sql.ErrNoRows {
		log.Printf("Erreur lors de la récupération de l'ancien avatar: %v", err)
	}

	// Uploader le nouvel avatar
	avatarURL, err := awsService.UploadAvatar(req.AvatarBase64)
	if err != nil {
		log.Printf("Erreur lors de l'upload de l'avatar: %v", err)
		http.Error(w, "Erreur lors de l'upload de l'avatar", http.StatusInternalServerError)
		return
	}

	// Mettre à jour l'avatar dans la base de données
	_, err = config.DB.Exec(`
		UPDATE admins 
		SET avatar_url = $1 
		WHERE id = $2`,
		avatarURL,
		adminID,
	)

	if err != nil {
		log.Printf("Erreur lors de la mise à jour de l'avatar: %v", err)
		http.Error(w, "Erreur lors de la mise à jour de l'avatar", http.StatusInternalServerError)
		return
	}

	// Supprimer l'ancien avatar de S3 (si existe)
	if oldAvatarURL.Valid && oldAvatarURL.String != "" {
		err = awsService.DeleteImages([]string{oldAvatarURL.String})
		if err != nil {
			log.Printf("Erreur lors de la suppression de l'ancien avatar: %v", err)
			// On continue même si la suppression échoue
		}
	}

	log.Printf("Avatar mis à jour avec succès pour l'admin ID: %d", adminID)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":   "Avatar mis à jour avec succès",
		"avatarUrl": avatarURL,
	})
}

// DeleteAdminAvatarHandler permet à l'admin de supprimer son avatar
func DeleteAdminAvatarHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Début de la suppression de l'avatar admin")

	// Récupérer l'ID de l'admin depuis le contexte
	adminID, ok := r.Context().Value(adminIDContextKey).(int)
	if !ok {
		log.Println("ID admin non trouvé dans le contexte")
		http.Error(w, "Non autorisé", http.StatusUnauthorized)
		return
	}

	// Récupérer l'URL de l'avatar actuel
	var avatarURL sql.NullString
	err := config.DB.QueryRow(`
		SELECT avatar_url 
		FROM admins 
		WHERE id = $1`,
		adminID,
	).Scan(&avatarURL)

	if err != nil {
		log.Printf("Erreur lors de la récupération de l'avatar: %v", err)
		http.Error(w, "Erreur de base de données", http.StatusInternalServerError)
		return
	}

	// Mettre à jour la base de données (NULL)
	_, err = config.DB.Exec(`
		UPDATE admins 
		SET avatar_url = NULL 
		WHERE id = $1`,
		adminID,
	)

	if err != nil {
		log.Printf("Erreur lors de la suppression de l'avatar: %v", err)
		http.Error(w, "Erreur lors de la suppression de l'avatar", http.StatusInternalServerError)
		return
	}

	// Supprimer l'image de S3 (si existe)
	if avatarURL.Valid && avatarURL.String != "" {
		awsService, err := services.NewAWSService()
		if err != nil {
			log.Printf("Erreur lors de l'initialisation du service AWS: %v", err)
		} else {
			err = awsService.DeleteImages([]string{avatarURL.String})
			if err != nil {
				log.Printf("Erreur lors de la suppression de l'avatar de S3: %v", err)
			}
		}
	}

	log.Printf("Avatar supprimé avec succès pour l'admin ID: %d", adminID)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Avatar supprimé avec succès",
	})
}
