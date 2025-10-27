package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"kivendi-backend/config"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"golang.org/x/crypto/bcrypt"
)

// NOTE : Le middleware d'authentification admin valide le JWT, récupère l'admin depuis la table `admins`
// et injecte son ID et son Rôle dans le contexte avec ces clés.
// Les clés de contexte sont définies dans auth_admin.go

// --- Structures de Réponse ---
// Créer un type personnalisé pour les dates nullables
type NullableTime struct {
	sql.NullTime
}

// Implémenter MarshalJSON pour retourner null ou la date ISO
func (nt NullableTime) MarshalJSON() ([]byte, error) {
	if !nt.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(nt.Time.Format(time.RFC3339))
}

// StaffMemberResponse est la structure JSON retournée.
// Elle omet les champs sensibles comme password_hash.
type StaffMemberResponse struct {
	ID        int          `json:"id"`
	Email     string       `json:"email"`
	FirstName string       `json:"first_name"`
	LastName  string       `json:"last_name"`
	Role      string       `json:"role"`
	IsActive  bool         `json:"is_active"`
	CreatedAt NullableTime `json:"created_at"` // ← Changé
	CreatedBy *int         `json:"created_by"`
	LastLogin NullableTime `json:"last_login"` // ← Changé
}

// --- Helpers ---

// httpError est un helper pour logger les erreurs et envoyer une réponse JSON
func httpError(w http.ResponseWriter, message string, code int, err error) {
	if err != nil {
		log.Printf("ERREUR Admin Mgmt: %s: %v", message, err)
	} else {
		log.Printf("ERREUR Admin Mgmt: %s", message)
	}
	http.Error(w, fmt.Sprintf(`{"error": "%s"}`, message), code)
}

// getRequestingAdmin récupère l'ID et le Rôle de l'admin qui fait la requête
func getRequestingAdmin(r *http.Request) (int, string, error) {
	idVal := r.Context().Value(adminIDContextKey)
	roleVal := r.Context().Value(adminRoleContextKey)

	adminID, ok := idVal.(int)
	if !ok {
		return 0, "", fmt.Errorf("ID admin non trouvé dans le contexte")
	}

	adminRole, ok := roleVal.(string)
	if !ok || (adminRole != "admin" && adminRole != "moderateur") {
		return 0, "", fmt.Errorf("rôle admin non trouvé ou invalide dans le contexte")
	}

	return adminID, adminRole, nil
}

// getTargetAdminInfo récupère le rôle et le statut d'un admin cible depuis la DB
func getTargetAdminInfo(targetAdminID int) (role string, isActive bool, err error) {
	err = config.DB.QueryRow("SELECT role, is_active FROM admins WHERE id = $1", targetAdminID).Scan(&role, &isActive)
	if err == sql.ErrNoRows {
		err = fmt.Errorf("l'utilisateur admin cible n'existe pas")
	}
	return
}

// hashPassword génère un hash bcrypt pour un mot de passe
func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 12) // Coût de 12
	return string(bytes), err
}

// --- Handlers ---

// CreateStaffMemberHandler crée un nouvel admin ou modérateur
func CreateStaffMemberHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	requestingAdminID, requestingAdminRole, err := getRequestingAdmin(r)
	if err != nil {
		httpError(w, "Accès non autorisé", http.StatusUnauthorized, err)
		return
	}

	var req struct {
		Email     string `json:"email"`
		Password  string `json:"password"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Role      string `json:"role"` // "admin" ou "moderateur"
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, "Corps de requête invalide", http.StatusBadRequest, err)
		return
	}

	// --- Validation des données ---
	req.Role = strings.ToLower(req.Role)
	if req.Role != "admin" && req.Role != "moderateur" {
		httpError(w, "Rôle invalide. Doit être 'admin' ou 'moderateur'.", http.StatusBadRequest, nil)
		return
	}
	if len(req.Password) < 8 {
		httpError(w, "Le mot de passe doit contenir au moins 8 caractères", http.StatusBadRequest, nil)
		return
	}
	if req.Email == "" || req.FirstName == "" || req.LastName == "" {
		httpError(w, "Email, nom et prénom sont requis", http.StatusBadRequest, nil)
		return
	}

	// --- Règles de Sécurité ---
	// 1. Un modérateur ne peut pas créer un compte 'admin'
	if requestingAdminRole == "moderateur" && req.Role == "admin" {
		httpError(w, "Les modérateurs ne sont pas autorisés à créer des administrateurs", http.StatusForbidden, nil)
		return
	}

	// --- Logique de Création ---
	passwordHash, err := hashPassword(req.Password)
	if err != nil {
		httpError(w, "Erreur lors du hachage du mot de passe", http.StatusInternalServerError, err)
		return
	}

	var newAdmin StaffMemberResponse
	query := `
		INSERT INTO admins (email, password_hash, first_name, last_name, role, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, email, first_name, last_name, role, is_active, created_at, created_by
	`
	err = config.DB.QueryRow(
		query,
		req.Email, passwordHash, req.FirstName, req.LastName, req.Role, requestingAdminID,
	).Scan(
		&newAdmin.ID, &newAdmin.Email, &newAdmin.FirstName, &newAdmin.LastName,
		&newAdmin.Role, &newAdmin.IsActive, &newAdmin.CreatedAt, &newAdmin.CreatedBy,
	)

	if err != nil {
		// Gérer les contraintes d'unicité (ex: email déjà pris)
		if strings.Contains(err.Error(), "admins_email_key") {
			httpError(w, "Un compte avec cet email existe déjà", http.StatusConflict, err)
		} else {
			httpError(w, "Erreur lors de la création du compte", http.StatusInternalServerError, err)
		}
		return
	}

	log.Printf("Admin %d (%s) a créé un nouveau membre du staff: %d (%s)",
		requestingAdminID, requestingAdminRole, newAdmin.ID, newAdmin.Role)

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(newAdmin)
}

// GetAllStaffHandler récupère la liste de tous les administrateurs et modérateurs
func GetAllStaffHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Seuls les admins et modérateurs peuvent voir la liste du staff
	_, _, err := getRequestingAdmin(r)
	if err != nil {
		httpError(w, "Accès non autorisé", http.StatusUnauthorized, err)
		return
	}

	rows, err := config.DB.Query(`
		SELECT id, email, first_name, last_name, role, is_active, created_at, created_by, last_login 
		FROM admins 
		ORDER BY role, last_name
	`)
	if err != nil {
		httpError(w, "Erreur lors de la récupération du staff", http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()

	var staff []StaffMemberResponse
	for rows.Next() {
		var s StaffMemberResponse
		var createdBy sql.NullInt64 // Utiliser sql.NullInt64 pour `created_by`

		err := rows.Scan(
			&s.ID, &s.Email, &s.FirstName, &s.LastName, &s.Role, &s.IsActive,
			&s.CreatedAt, &createdBy, &s.LastLogin,
		)
		if err != nil {
			httpError(w, "Erreur lors de la lecture du staff", http.StatusInternalServerError, err)
			return
		}

		if createdBy.Valid {
			val := int(createdBy.Int64)
			s.CreatedBy = &val
		}

		staff = append(staff, s)
	}

	json.NewEncoder(w).Encode(staff)
}

// UpdateStaffProfileHandler met à jour les informations de profil (nom, email)
func UpdateStaffProfileHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	requestingAdminID, requestingAdminRole, err := getRequestingAdmin(r)
	if err != nil {
		httpError(w, "Accès non autorisé", http.StatusUnauthorized, err)
		return
	}

	vars := mux.Vars(r)
	targetUserID, err := strconv.Atoi(vars["id"])
	if err != nil {
		httpError(w, "ID utilisateur invalide", http.StatusBadRequest, err)
		return
	}

	var req struct {
		Email     string `json:"email"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, "Corps de requête invalide", http.StatusBadRequest, err)
		return
	}

	// --- Règles de Sécurité ---
	// 1. Un modérateur ne peut pas modifier le profil d'un 'admin'
	if requestingAdminRole == "moderateur" && targetUserID != requestingAdminID {
		targetRole, _, err := getTargetAdminInfo(targetUserID)
		if err != nil {
			httpError(w, "Utilisateur cible non trouvé", http.StatusNotFound, err)
			return
		}
		if targetRole == "admin" {
			httpError(w, "Les modérateurs ne peuvent pas modifier le profil des administrateurs", http.StatusForbidden, nil)
			return
		}
	}
	// Note : On autorise un admin à modifier son *propre* profil (mais pas son rôle/statut)

	// --- Logique de Mise à Jour ---
	query := `
		UPDATE admins 
		SET email = $1, first_name = $2, last_name = $3, updated_at = CURRENT_TIMESTAMP
		WHERE id = $4
		RETURNING id, email, first_name, last_name, role, is_active, created_at, created_by, last_login
	`
	var updatedAdmin StaffMemberResponse
	var createdBy sql.NullInt64

	err = config.DB.QueryRow(query, req.Email, req.FirstName, req.LastName, targetUserID).Scan(
		&updatedAdmin.ID, &updatedAdmin.Email, &updatedAdmin.FirstName, &updatedAdmin.LastName,
		&updatedAdmin.Role, &updatedAdmin.IsActive, &updatedAdmin.CreatedAt, &createdBy, &updatedAdmin.LastLogin,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			httpError(w, "Utilisateur cible non trouvé", http.StatusNotFound, err)
		} else if strings.Contains(err.Error(), "admins_email_key") {
			httpError(w, "Cet email est déjà utilisé par un autre compte", http.StatusConflict, err)
		} else {
			httpError(w, "Erreur lors de la mise à jour du profil", http.StatusInternalServerError, err)
		}
		return
	}

	if createdBy.Valid {
		val := int(createdBy.Int64)
		updatedAdmin.CreatedBy = &val
	}

	log.Printf("Admin %d a mis à jour le profil de l'utilisateur %d", requestingAdminID, targetUserID)
	json.NewEncoder(w).Encode(updatedAdmin)
}

// UpdateStaffPermissionsHandler met à jour le rôle et le statut (is_active)
func UpdateStaffPermissionsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	requestingAdminID, requestingAdminRole, err := getRequestingAdmin(r)
	if err != nil {
		httpError(w, "Accès non autorisé", http.StatusUnauthorized, err)
		return
	}

	vars := mux.Vars(r)
	targetUserID, err := strconv.Atoi(vars["id"])
	if err != nil {
		httpError(w, "ID utilisateur invalide", http.StatusBadRequest, err)
		return
	}

	var req struct {
		Role     string `json:"role"`
		IsActive bool   `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, "Corps de requête invalide", http.StatusBadRequest, err)
		return
	}

	req.Role = strings.ToLower(req.Role)
	if req.Role != "admin" && req.Role != "moderateur" {
		httpError(w, "Rôle invalide. Doit être 'admin' ou 'moderateur'.", http.StatusBadRequest, nil)
		return
	}

	// --- Règles de Sécurité ---
	// 1. On ne peut pas modifier le Super Admin (ID 1)
	if targetUserID == 1 {
		httpError(w, "Impossible de modifier les permissions du Super Administrateur", http.StatusForbidden, nil)
		return
	}
	// 2. On ne peut pas modifier ses propres permissions (rôle ou statut)
	if targetUserID == requestingAdminID {
		httpError(w, "Vous ne pouvez pas modifier vos propres permissions", http.StatusForbidden, nil)
		return
	}

	// 3. Un modérateur ne peut pas modifier les permissions d'un admin, ni promouvoir un autre en admin
	if requestingAdminRole == "moderateur" {
		targetRole, _, err := getTargetAdminInfo(targetUserID)
		if err != nil {
			httpError(w, "Utilisateur cible non trouvé", http.StatusNotFound, err)
			return
		}
		// Règle A: Un mod ne peut pas promouvoir un autre en 'admin'
		if req.Role == "admin" {
			httpError(w, "Les modérateurs ne peuvent pas promouvoir en administrateur", http.StatusForbidden, nil)
			return
		}
		// Règle B: Un mod ne peut pas modifier un 'admin' existant (même pour le désactiver)
		if targetRole == "admin" {
			httpError(w, "Les modérateurs ne peuvent pas modifier les permissions des administrateurs", http.StatusForbidden, nil)
			return
		}
	}

	// --- Logique de Mise à Jour ---
	query := `
		UPDATE admins 
		SET role = $1, is_active = $2, updated_at = CURRENT_TIMESTAMP
		WHERE id = $3
	`
	result, err := config.DB.Exec(query, req.Role, req.IsActive, targetUserID)
	if err != nil {
		httpError(w, "Erreur lors de la mise à jour des permissions", http.StatusInternalServerError, err)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		httpError(w, "Utilisateur cible non trouvé", http.StatusNotFound, nil)
		return
	}

	log.Printf("Admin %d a mis à jour les permissions de l'utilisateur %d (Rôle: %s, Actif: %v)",
		requestingAdminID, targetUserID, req.Role, req.IsActive)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Permissions mises à jour avec succès"})
}

// DeleteStaffMemberHandler supprime un admin ou un modérateur
func DeleteStaffMemberHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	requestingAdminID, requestingAdminRole, err := getRequestingAdmin(r)
	if err != nil {
		httpError(w, "Accès non autorisé", http.StatusUnauthorized, err)
		return
	}

	vars := mux.Vars(r)
	targetUserID, err := strconv.Atoi(vars["id"])
	if err != nil {
		httpError(w, "ID utilisateur invalide", http.StatusBadRequest, err)
		return
	}

	// --- Règles de Sécurité ---
	// 1. Règle Explicite : On ne peut pas supprimer le Super Admin (ID 1)
	if targetUserID == 1 {
		httpError(w, "Impossible de supprimer le Super Administrateur (ID 1)", http.StatusForbidden, nil)
		return
	}
	// 2. Règle de sécurité : On ne peut pas se supprimer soi-même
	if targetUserID == requestingAdminID {
		httpError(w, "Vous ne pouvez pas supprimer votre propre compte", http.StatusForbidden, nil)
		return
	}

	// 3. Règle Explicite : Un modérateur ne peut pas supprimer un admin
	targetRole, _, err := getTargetAdminInfo(targetUserID)
	if err != nil {
		httpError(w, "Utilisateur cible non trouvé", http.StatusNotFound, err)
		return
	}

	if requestingAdminRole == "moderateur" && targetRole == "admin" {
		httpError(w, "Les modérateurs ne sont pas autorisés à supprimer les administrateurs", http.StatusForbidden, nil)
		return
	}

	// --- Logique de Suppression ---
	// Note : La contrainte `created_by` (ON DELETE SET NULL) sera gérée par PostgreSQL
	query := "DELETE FROM admins WHERE id = $1"
	result, err := config.DB.Exec(query, targetUserID)
	if err != nil {
		httpError(w, "Erreur lors de la suppression de l'utilisateur", http.StatusInternalServerError, err)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		httpError(w, "Utilisateur non trouvé", http.StatusNotFound, nil)
		return
	}

	log.Printf("Admin %d a SUPPRIMÉ l'utilisateur %d (Rôle: %s)", requestingAdminID, targetUserID, targetRole)
	w.WriteHeader(http.StatusNoContent)
}
