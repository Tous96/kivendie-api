package middleware

import (
	"database/sql"
	"encoding/json"
	"kivendi-backend/config"
	"log"
	"net/http"
	"strings"

	"github.com/lib/pq"
)

// MaintenanceMode représente le statut de maintenance
type MaintenanceMode struct {
	IsActive           bool     `json:"is_active"`
	Title              string   `json:"title"`
	Message            string   `json:"message"`
	AllowAdminAccess   bool     `json:"allow_admin_access"`
	AllowedIPAddresses []string `json:"allowed_ip_addresses"`
}

// CheckMaintenanceMode vérifie si l'application est en mode maintenance
// et bloque l'accès si nécessaire (sauf pour les admins et IPs autorisées)
func CheckMaintenanceMode(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Ne pas appliquer le middleware pour certaines routes critiques
		exemptedPaths := []string{
			"/api/v1/admin/login",
			"/api/v1/admin/refresh",
			"/api/v1/maintenance/status",
			"/api/v1/settings/app",
		}

		for _, path := range exemptedPaths {
			if r.URL.Path == path {
				next.ServeHTTP(w, r)
				return
			}
		}

		// Récupérer le statut de maintenance
		var maintenance MaintenanceMode
		var allowedIPs pq.StringArray

		query := `
			SELECT is_active, title, message, allow_admin_access, allowed_ip_addresses
			FROM maintenance_mode
			ORDER BY id DESC
			LIMIT 1
		`

		err := config.DB.QueryRow(query).Scan(
			&maintenance.IsActive,
			&maintenance.Title,
			&maintenance.Message,
			&maintenance.AllowAdminAccess,
			&allowedIPs,
		)

		// Si erreur ou pas en maintenance, laisser passer
		if err != nil {
			if err != sql.ErrNoRows {
				log.Printf("Erreur lors de la vérification du mode maintenance: %v", err)
			}
			next.ServeHTTP(w, r)
			return
		}

		maintenance.AllowedIPAddresses = []string(allowedIPs)

		// Si pas en maintenance, laisser passer
		if !maintenance.IsActive {
			next.ServeHTTP(w, r)
			return
		}

		// L'app est en maintenance, vérifier les exceptions

		// 1. Vérifier si c'est un admin et si l'accès admin est autorisé
		if maintenance.AllowAdminAccess {
			// Vérifier si la route est une route admin
			if strings.HasPrefix(r.URL.Path, "/api/v1/admin") {
				// Laisser passer les admins authentifiés
				next.ServeHTTP(w, r)
				return
			}
		}

		// 2. Vérifier l'IP de l'utilisateur
		clientIP := getClientIP(r)
		for _, allowedIP := range maintenance.AllowedIPAddresses {
			if clientIP == allowedIP {
				next.ServeHTTP(w, r)
				return
			}
		}

		// 3. Si aucune exception ne s'applique, bloquer l'accès
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)

		response := map[string]interface{}{
			"maintenance_mode": true,
			"title":            maintenance.Title,
			"message":          maintenance.Message,
		}

		json.NewEncoder(w).Encode(response)
	})
}

// getClientIP récupère l'adresse IP du client
func getClientIP(r *http.Request) string {
	// Vérifier les headers de proxy
	ip := r.Header.Get("X-Real-IP")
	if ip != "" {
		return ip
	}

	ip = r.Header.Get("X-Forwarded-For")
	if ip != "" {
		// X-Forwarded-For peut contenir plusieurs IPs séparées par des virgules
		// Prendre la première
		ips := strings.Split(ip, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Sinon, utiliser RemoteAddr
	ip = r.RemoteAddr
	// Supprimer le port si présent
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}

	return ip
}
