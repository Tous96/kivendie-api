package routes

import (
	"kivendi-backend/handlers"
	"net/http"

	"github.com/gorilla/mux"
)

// SetupRoutes configure le routeur et définit les endpoints de l'API.
func SetupRoutes() *mux.Router {
	router := mux.NewRouter()

	// Création d'un sous-routeur pour la version 1 de l'API
	apiV1 := router.PathPrefix("/api/v1").Subrouter()

	apiV1.HandleFunc("/register", handlers.RegisterHandler).Methods("POST")
	apiV1.HandleFunc("/verify", handlers.VerifyHandler).Methods("POST")
	apiV1.HandleFunc("/resend-verification", handlers.ResendVerificationCodeHandler).Methods("POST")
	apiV1.HandleFunc("/login", handlers.LoginHandler).Methods("POST")
	apiV1.HandleFunc("/refresh", handlers.RefreshHandler).Methods("POST")
	apiV1.Handle("/profile", handlers.ValidateToken(http.HandlerFunc(handlers.UpdateProfileHandler))).Methods("PUT")

	apiV1.HandleFunc("/auth/google/login", handlers.GoogleLoginHandler).Methods("GET")
	apiV1.HandleFunc("/auth/google/callback", handlers.GoogleCallbackHandler).Methods("GET")
	// HANDLER POUR LE FLUX MOBILE (Flutter)
	apiV1.HandleFunc("/google-mobile-login", handlers.GoogleMobileLoginHandler).Methods("POST")
	// Routes pour la réinitialisation du mot de passe
	apiV1.HandleFunc("/forgot-password", handlers.ForgotPasswordHandler).Methods("POST")
	apiV1.HandleFunc("/reset-password", handlers.ResetPasswordHandler).Methods("POST")
	// Dans routes.go, ajoutez ces lignes dans la fonction SetupRoutes()

	// 👇 ROUTES POUR LES PARAMÈTRES (SETTINGS) 👇
	// Route pour récupérer les utilisateurs bloqués, protégée par le middleware JWT
	apiV1.Handle("/settings/blocked-users", handlers.ValidateToken(http.HandlerFunc(handlers.GetBlockedUsersHandler))).Methods("GET")

	// Route pour débloquer un utilisateur, protégée par le middleware JWT
	apiV1.Handle("/settings/blocked-users/{userID}", handlers.ValidateToken(http.HandlerFunc(handlers.UnblockUserByIDHandler))).Methods("DELETE")

	// Route pour supprimer le compte utilisateur, protégée par le middleware JWT
	apiV1.Handle("/settings/account", handlers.ValidateToken(http.HandlerFunc(handlers.DeleteAccountHandler))).Methods("DELETE")

	// Route pour récupérer les paramètres utilisateur, protégée par le middleware JWT
	apiV1.Handle("/settings", handlers.ValidateToken(http.HandlerFunc(handlers.GetUserSettingsHandler))).Methods("GET")

	// Route pour mettre à jour les paramètres utilisateur, protégée par le middleware JWT
	apiV1.Handle("/settings", handlers.ValidateToken(http.HandlerFunc(handlers.UpdateUserSettingsHandler))).Methods("PUT")

	// 👇 ROUTES POUR LES NOTIFICATIONS 👇
	// Préférences de notification
	apiV1.Handle("/notifications/preferences", handlers.ValidateToken(http.HandlerFunc(handlers.GetNotificationPreferencesHandler))).Methods("GET")
	apiV1.Handle("/notifications/preferences", handlers.ValidateToken(http.HandlerFunc(handlers.UpdateNotificationPreferencesHandler))).Methods("PUT")

	// Enregistrer un token de notification push (FCM, APNS)
	apiV1.Handle("/notifications/register-token", handlers.ValidateToken(http.HandlerFunc(handlers.RegisterDeviceTokenHandler))).Methods("POST")

	// 👇 ROUTES POUR LE SUPPORT ET CONTACT 👇
	// Route pour la soumission du formulaire de contact (accès public)
	apiV1.HandleFunc("/support/contact", handlers.SubmitContactFormHandler).Methods("POST") // NOUVELLE ROUTE ICI
	// Route pour récupérer les informations de contact/support (accès public)
	apiV1.HandleFunc("/support/contact", handlers.GetSupportContactHandler).Methods("GET") // NOUVELLE ROUTE ICI

	// 👇 ROUTES POUR LES TERMES ET CONDITIONS 👇
	// Route publique pour récupérer les termes et conditions actifs
	apiV1.HandleFunc("/terms-conditions", handlers.GetActiveTermsHandler).Methods("GET")

	// Gestion des notifications
	apiV1.Handle("/notifications", handlers.ValidateToken(http.HandlerFunc(handlers.GetNotificationsHandler))).Methods("GET")
	apiV1.Handle("/notifications/{notificationID}", handlers.ValidateToken(http.HandlerFunc(handlers.MarkNotificationAsReadHandler))).Methods("PATCH")
	apiV1.Handle("/notifications/mark-all-read", handlers.ValidateToken(http.HandlerFunc(handlers.MarkAllNotificationsAsReadHandler))).Methods("PATCH")
	apiV1.Handle("/notifications/{notificationID}", handlers.ValidateToken(http.HandlerFunc(handlers.DeleteNotificationHandler))).Methods("DELETE")
	apiV1.Handle("/notifications/unread-count", handlers.ValidateToken(http.HandlerFunc(handlers.GetUnreadNotificationCountHandler))).Methods("GET")

	// Routes pour les catégories et les sous-catégories
	apiV1.HandleFunc("/categories", handlers.GetCategoriesWithSubCategories).Methods("GET")
	apiV1.HandleFunc("/ads/e/{adID}", handlers.GetAdDetailsHandler).Methods("GET")
	apiV1.HandleFunc("/ads/cities", handlers.GetAvailableCitiesHandler).Methods("GET")
	// Nouvelle route pour récupérer toutes les annonces (pour un tableau de bord admin par exemple)
	apiV1.HandleFunc("/ads/all", handlers.GetAllAdsHandler).Methods("GET")
	// Route pour incrémenter le nombre de vues d'une annonce
	apiV1.HandleFunc("/ads/{adID}/views", handlers.IncrementAdViewsHandler).Methods("POST")

	// NOUVELLE ROUTE: Récupérer les annonces par catégorie
	apiV1.HandleFunc("/ads/category/{categoryID}", handlers.GetAdsByCategoryHandler).Methods("GET")

	// Nouvelle route pour récupérer les annonces de l'utilisateur connecté
	apiV1.Handle("/ads/me", handlers.ValidateToken(http.HandlerFunc(handlers.GetUserAdsHandler))).Methods("GET")

	// Route pour marquer une annonce comme vendue, protégée par le middleware JWT
	apiV1.Handle("/ads/{adID}/mark-sold", handlers.ValidateToken(http.HandlerFunc(handlers.MarkAdAsSoldHandler))).Methods("POST")

	// Route pour démarquer une annonce comme vendue (réactiver), protégée par le middleware JWT
	apiV1.Handle("/ads/{adID}/unmark-sold", handlers.ValidateToken(http.HandlerFunc(handlers.UnmarkAdAsSoldHandler))).Methods("DELETE")

	// Route pour récupérer toutes les annonces vendues de l'utilisateur, protégée par le middleware JWT
	apiV1.Handle("/ads/sold", handlers.ValidateToken(http.HandlerFunc(handlers.GetSoldAdsHandler))).Methods("GET")

	// Route pour supprimer une annonce (protégée par le middleware JWT)
	apiV1.Handle("/ads/{adID}", handlers.ValidateToken(http.HandlerFunc(handlers.DeleteAdHandler))).Methods("DELETE")

	// Route pour modifier une annonce (protégée par le middleware JWT)
	apiV1.Handle("/ads/{adID}", handlers.ValidateToken(http.HandlerFunc(handlers.EditAdHandler))).Methods("PUT")

	// Nouvelle route pour les annonces similaires
	apiV1.HandleFunc("/ads/e/{adID}/similar", handlers.GetSimilarAdsHandler).Methods("GET")

	// Route pour récupérer le profil d'un vendeur et ses articles validés (accès public)
	apiV1.HandleFunc("/sellers/{userID}", handlers.GetSellerProfileHandler).Methods("GET")

	// Route pour la création d'annonces, protégée par le middleware JWT
	apiV1.Handle("/ads", handlers.ValidateToken(http.HandlerFunc(handlers.CreateAdHandler))).Methods("POST")

	// Nouvelle route pour récupérer les annonces validées (accès public)
	apiV1.HandleFunc("/ads/validated", handlers.GetValidatedAdsHandler).Methods("GET")
	// Route pour la recherche d'annonces (accès public)
	apiV1.HandleFunc("/ads/search", handlers.SearchAdsHandler).Methods("GET")

	// Route pour le deep linking
	apiV1.HandleFunc("/share/ad/{adID}", handlers.DeepLinkHandler).Methods("GET")
	// Note : Le token est un paramètre query (?token=...), pas un segment de path
	apiV1.HandleFunc("/reset-password", handlers.PasswordResetLinkHandler).Methods("GET")

	// Routes pour la gestion des favoris, protégées par le middleware JWT
	apiV1.Handle("/favorites", handlers.ValidateToken(http.HandlerFunc(handlers.AddFavoriteHandler))).Methods("POST")
	apiV1.Handle("/favorites/{adID}", handlers.ValidateToken(http.HandlerFunc(handlers.RemoveFavoriteHandler))).Methods("DELETE")
	apiV1.Handle("/favorites", handlers.ValidateToken(http.HandlerFunc(handlers.GetFavoritesHandler))).Methods("GET")

	// Route protégée par le middleware JWT
	apiV1.Handle("/profile", handlers.ValidateToken(http.HandlerFunc(handlers.ProfileHandler))).Methods("GET")

	// Routes publiques pour les offres de boost
	apiV1.HandleFunc("/boost-offers", handlers.GetBoostOffersHandler).Methods("GET")
	apiV1.HandleFunc("/boost-offers/{offerID}", handlers.GetBoostOfferDetailsHandler).Methods("GET")

	// Route publique pour récupérer les annonces boostées
	apiV1.HandleFunc("/ads/boosted", handlers.GetBoostedAdsHandler).Methods("GET")

	// Route publique pour vérifier le statut de boost d'une annonce
	apiV1.HandleFunc("/ads/{adID}/boost-status", handlers.CheckAdBoostStatusHandler).Methods("GET")

	// Routes protégées (nécessitent une authentification)
	// Acheter un boost pour une annonce
	apiV1.Handle("/ads/{adID}/boost", handlers.ValidateToken(http.HandlerFunc(handlers.PurchaseBoostHandler))).Methods("POST")

	// Récupérer l'historique des boosts de l'utilisateur
	apiV1.Handle("/boosts/history", handlers.ValidateToken(http.HandlerFunc(handlers.GetUserBoostHistoryHandler))).Methods("GET")

	// 👇 NOUVELLE ROUTE: WEBHOOK KKIAPAY 👇
	// Route webhook pour recevoir les notifications de KKiaPay (accès public mais sécurisé par signature)
	apiV1.HandleFunc("/webhooks/kkiapay", handlers.KKiaPayWebhookHandler).Methods("POST")

	// Route pour voir les logs webhook (protégée, admin uniquement)
	apiV1.Handle("/webhooks/kkiapay/logs", handlers.ValidateToken(http.HandlerFunc(handlers.GetWebhookLogsHandler))).Methods("GET")

	// 👇 ROUTES POUR LE CHAT 👇
	// Route pour créer ou obtenir une conversation, protégée par le middleware JWT
	apiV1.Handle("/conversations/ad/{adID}", handlers.ValidateToken(http.HandlerFunc(handlers.GetOrCreateConversation))).Methods("POST")

	// Route pour récupérer l'historique des messages, protégée par le middleware JWT
	apiV1.Handle("/conversations/{conversationID}/messages", handlers.ValidateToken(http.HandlerFunc(handlers.GetConversationHistory))).Methods("GET")

	// Route pour la liste des conversations, protégée par le middleware JWT
	apiV1.Handle("/conversations/list", handlers.ValidateToken(http.HandlerFunc(handlers.GetConversationListHandler))).Methods("GET")

	// NOUVELLE ROUTE : pour marquer plusieurs messages comme lus en une seule requête PATCH.
	apiV1.Handle("/conversations/{conversationID}/messages/read", handlers.ValidateToken(http.HandlerFunc(handlers.MarkMessagesAsReadHandler))).Methods("PATCH")

	// 👇 NOUVELLES ROUTES POUR LE BLOCAGE ET SIGNALEMENT 👇
	// Route pour bloquer un utilisateur
	apiV1.Handle("/conversations/{conversationID}/block", handlers.ValidateToken(http.HandlerFunc(handlers.BlockUserHandler))).Methods("POST")

	// Route pour débloquer un utilisateur
	apiV1.Handle("/conversations/{conversationID}/unblock", handlers.ValidateToken(http.HandlerFunc(handlers.UnblockUserHandler))).Methods("DELETE")

	// Route pour signaler un utilisateur
	apiV1.Handle("/conversations/{conversationID}/report", handlers.ValidateToken(http.HandlerFunc(handlers.ReportUserHandler))).Methods("POST")

	// Route pour vérifier le statut de blocage
	apiV1.Handle("/conversations/{conversationID}/block-status", handlers.ValidateToken(http.HandlerFunc(handlers.CheckBlockStatusHandler))).Methods("GET")

	// Route pour le WebSocket.
	router.Handle("/ws/chat/{conversationID}", handlers.ValidateToken(http.HandlerFunc(handlers.WebSocketHandler)))

	// Nouvelle route pour le WebSocket de notifications génériques
	router.Handle("/ws/notifications", handlers.ValidateToken(http.HandlerFunc(handlers.HandleNotificationsWebSocket)))

	// ==============================================================
	// ROUTES PUBLIQUES - PARAMÈTRES D'APPLICATION & MAINTENANCE
	// ==============================================================

	// Route publique pour récupérer les paramètres de l'application
	apiV1.HandleFunc("/settings/app", handlers.GetPublicAppSettingsHandler).Methods("GET")

	// Route publique pour vérifier le statut de maintenance
	apiV1.HandleFunc("/maintenance/status", handlers.GetMaintenanceStatusHandler).Methods("GET")

	// ===================================================================
	// 👇 ROUTES POUR L'ESPACE ADMINISTRATION 👇
	// ===================================================================

	// --- Routes publiques pour l'authentification de l'admin ---
	// Pas de middleware de validation de token ici car l'admin n'est pas encore connecté.
	apiV1.HandleFunc("/admin/login", handlers.AdminLoginHandler).Methods("POST")
	apiV1.HandleFunc("/admin/refresh", handlers.AdminRefreshHandler).Methods("POST")

	// --- Routes protégées pour l'admin ---
	// Un sous-routeur pour les routes admin qui nécessitent une authentification
	adminRoutes := apiV1.PathPrefix("/admin").Subrouter()
	adminRoutes.Use(handlers.ValidateAdminToken, handlers.RequireModeratorOrAdmin)

	// Route pour récupérer le profil de l'admin connecté
	adminRoutes.HandleFunc("/profile", handlers.GetAdminProfileHandler).Methods("GET")
	adminRoutes.HandleFunc("/profile/password", handlers.UpdateAdminPasswordHandler).Methods("PUT")
	adminRoutes.HandleFunc("/profile/avatar", handlers.UpdateAdminAvatarHandler).Methods("PUT")
	adminRoutes.HandleFunc("/profile/avatar", handlers.DeleteAdminAvatarHandler).Methods("DELETE")

	// Route pour les statistiques du tableau de bord
	adminRoutes.HandleFunc("/stats/dashboard", handlers.GetDashboardStatsHandler).Methods("GET")

	// 👇 NOUVELLE ROUTE POUR LES CATÉGORIES (ADMIN) 👇
	adminRoutes.HandleFunc("/categories", handlers.GetCategoriesForAdminHandler).Methods("GET")
	// Route pour récupérer toutes les annonces pour le panel admin
	adminRoutes.HandleFunc("/ads", handlers.GetAllAdsForAdminHandler).Methods("GET")

	// ===================================================================
	// === CORRECTION APPLIQUÉE ICI ===
	// La route statique "/ads/boostable" DOIT être déclarée AVANT
	// la route dynamique "/ads/{adID}" pour éviter un conflit.
	// ===================================================================

	// Récupérer les annonces "boostables" (non-boostées)
	adminRoutes.HandleFunc("/ads/boostable", handlers.GetNonBoostedAdsForAdminHandler).Methods("GET")

	// NOUVELLE ROUTE POUR LES DÉTAILS D'UNE ANNONCE (Dynamique)
	adminRoutes.HandleFunc("/ads/{adID}", handlers.GetAdDetailsForAdminHandler).Methods("GET")

	// ============== NOUVELLES ROUTES DE MODÉRATION ==============
	adminRoutes.HandleFunc("/ads/{adID}/validate", handlers.ValidateAdHandler).Methods("POST")
	adminRoutes.HandleFunc("/ads/{adID}/reject", handlers.RejectAdHandler).Methods("POST")
	adminRoutes.HandleFunc("/ads/{adID}/deactivate", handlers.DeactivateAdHandler).Methods("POST")

	// 👇 NOUVELLE ROUTE DE MODIFICATION ADMIN 👇
	adminRoutes.HandleFunc("/ads/{adID}", handlers.EditAdForAdminHandler).Methods("PUT")

	adminRoutes.HandleFunc("/ads/{adID}", handlers.DeleteAdForAdminHandler).Methods("DELETE")

	// ============== ROUTES POUR LA GESTION DES BOOSTS (ADMIN) ==============
	adminRoutes.HandleFunc("/boosts", handlers.GetAllBoostedAdsAdminHandler).Methods("GET")
	adminRoutes.HandleFunc("/boosts/{boostID}/deactivate", handlers.DeactivateBoostAdminHandler).Methods("POST")
	adminRoutes.HandleFunc("/boosts/create", handlers.CreateBoostAdminHandler).Methods("POST")

	// ============== ROUTES POUR LA GESTION DES OFFRES DE BOOST ==============
	adminRoutes.HandleFunc("/boost-offers", handlers.GetAllBoostOffersForAdminHandler).Methods("GET")
	adminRoutes.HandleFunc("/boost-offers", handlers.CreateBoostOfferHandler).Methods("POST")
	adminRoutes.HandleFunc("/boost-offers/{offerID}", handlers.UpdateBoostOfferHandler).Methods("PUT")
	adminRoutes.HandleFunc("/boost-offers/{offerID}/toggle", handlers.ToggleBoostOfferStatusHandler).Methods("PATCH")
	adminRoutes.HandleFunc("/boost-offers/{offerID}", handlers.DeleteBoostOfferHandler).Methods("DELETE")
	adminRoutes.HandleFunc("/boost-offers/{offerID}/stats", handlers.GetBoostOfferStatsHandler).Methods("GET")

	// ============== ROUTES POUR LA GESTION DES CATÉGORIES ==============

	// Créer une nouvelle catégorie
	adminRoutes.HandleFunc("/categories", handlers.CreateCategoryHandler).Methods("POST")
	adminRoutes.HandleFunc("/categories/{categoryID}", handlers.UpdateCategoryHandler).Methods("PUT")
	adminRoutes.HandleFunc("/categories/{categoryID}", handlers.DeleteCategoryHandler).Methods("DELETE")

	// SOUS-CATÉGORIES
	adminRoutes.HandleFunc("/sub-categories", handlers.CreateSubCategoryHandler).Methods("POST")
	adminRoutes.HandleFunc("/sub-categories/{subCategoryID}", handlers.UpdateSubCategoryHandler).Methods("PUT")
	adminRoutes.HandleFunc("/sub-categories/{subCategoryID}", handlers.DeleteSubCategoryHandler).Methods("DELETE")

	// 👇 =================================================================
	// 👇 NOUVELLES ROUTES POUR LA GESTION DU STAFF (Table 'admins')
	// 👇 (AJOUTÉES ICI)
	// 👇 =================================================================
	adminRoutes.HandleFunc("/staff", handlers.CreateStaffMemberHandler).Methods("POST")
	adminRoutes.HandleFunc("/staff", handlers.GetAllStaffHandler).Methods("GET")
	adminRoutes.HandleFunc("/staff/{id:[0-9]+}/profile", handlers.UpdateStaffProfileHandler).Methods("PUT")
	adminRoutes.HandleFunc("/staff/{id:[0-V]+}/permissions", handlers.UpdateStaffPermissionsHandler).Methods("PUT")
	adminRoutes.HandleFunc("/staff/{id:[0-9]+}", handlers.DeleteStaffMemberHandler).Methods("DELETE")

	// 👇 =================================================================
	// 👇 NOUVELLES ROUTES POUR LA GESTION DES SIGNALEMENTS (Table 'user_reports')
	// 👇 =================================================================

	// Récupérer tous les signalements (filtrables par ?status=pending)
	adminRoutes.HandleFunc("/reports", handlers.GetReportsHandler).Methods("GET")
	adminRoutes.HandleFunc("/reports/{reportID:[0-9]+}", handlers.UpdateReportHandler).Methods("PATCH")

	// 👇 =================================================================
	// 👇 NOUVELLES ROUTES POUR LA GESTION DES TICKETS DE SUPPORT (ADMIN)
	// 👇 =================================================================

	// Récupérer tous les tickets (filtrables par ?status=Nouveau)
	adminRoutes.HandleFunc("/support-tickets", handlers.GetSupportTicketsHandler).Methods("GET")
	adminRoutes.HandleFunc("/support-tickets/{ticketID:[0-9]+}", handlers.UpdateSupportTicketHandler).Methods("PATCH")
	adminRoutes.HandleFunc("/support-tickets/{ticketID:[0-9]+}/reply", handlers.ReplySupportTicketHandler).Methods("POST")
	adminRoutes.HandleFunc("/support-tickets/{ticketID:[0-9]+}", handlers.DeleteSupportTicketHandler).Methods("DELETE")

	// 👇 =================================================================
	// 👇 NOUVELLES ROUTES POUR LA GESTION DES UTILISATEURS (Table 'users')
	// 👇 =================================================================

	// Récupérer tous les utilisateurs (avec filtres et pagination)
	adminRoutes.HandleFunc("/users", handlers.GetAllUsersHandler).Methods("GET")
	adminRoutes.HandleFunc("/users/{userID}", handlers.UpdateUserForAdminHandler).Methods("PUT")
	adminRoutes.HandleFunc("/users/{userID}/toggle-block", handlers.ToggleUserBlockStatusHandler).Methods("PATCH")
	adminRoutes.HandleFunc("/users/{userID}", handlers.DeleteUserForAdminHandler).Methods("DELETE")
	adminRoutes.HandleFunc("/users/{userID}/details", handlers.GetUserDetailsForAdminHandler).Methods("GET")

	// ============== NOUVELLES ROUTES POUR LA GESTION DES TERMES ET CONDITIONS ==============
	adminRoutes.HandleFunc("/terms-conditions", handlers.GetAllTermsForAdminHandler).Methods("GET")
	adminRoutes.HandleFunc("/terms-conditions/{termsID:[0-9]+}", handlers.GetTermsByIDForAdminHandler).Methods("GET")
	adminRoutes.HandleFunc("/terms-conditions", handlers.CreateTermsHandler).Methods("POST")
	adminRoutes.HandleFunc("/terms-conditions/{termsID:[0-9]+}", handlers.UpdateTermsHandler).Methods("PUT")
	adminRoutes.HandleFunc("/terms-conditions/{termsID:[0-9]+}/toggle", handlers.ToggleTermsStatusHandler).Methods("PATCH")
	adminRoutes.HandleFunc("/terms-conditions/{termsID:[0-9]+}", handlers.DeleteTermsHandler).Methods("DELETE")

	// ============== ROUTES POUR LA GESTION DES PAGES "À PROPOS" ==============

	// Route publique pour récupérer la page "À propos" active
	apiV1.HandleFunc("/about", handlers.GetActiveAboutPageHandler).Methods("GET")
	adminRoutes.HandleFunc("/about-pages", handlers.GetAllAboutPagesForAdminHandler).Methods("GET")
	adminRoutes.HandleFunc("/about-pages/{pageID:[0-9]+}", handlers.GetAboutPageByIDForAdminHandler).Methods("GET")
	adminRoutes.HandleFunc("/about-pages", handlers.CreateAboutPageHandler).Methods("POST")
	adminRoutes.HandleFunc("/about-pages/{pageID:[0-9]+}", handlers.UpdateAboutPageHandler).Methods("PUT")
	adminRoutes.HandleFunc("/about-pages/{pageID:[0-9]+}/toggle", handlers.ToggleAboutPageStatusHandler).Methods("PATCH")
	adminRoutes.HandleFunc("/about-pages/{pageID:[0-9]+}", handlers.DeleteAboutPageHandler).Methods("DELETE")

	// ============== ROUTES POUR LES TRANSACTIONS KKIAPAY (ADMIN UNIQUEMENT) ==============

	// Récupérer toutes les transactions avec pagination et filtres
	// Paramètres: ?page=1&limit=20&status=completed
	adminRoutes.HandleFunc("/transactions", handlers.GetAllTransactionsHandler).Methods("GET")
	adminRoutes.HandleFunc("/transactions/{transactionID:[0-9]+}", handlers.GetTransactionByIDHandler).Methods("GET")
	adminRoutes.HandleFunc("/transactions/stats", handlers.GetTransactionStatsHandler).Methods("GET")

	// ============== ROUTES POUR LES RAPPORTS ET STATISTIQUES (ADMIN) ==============

	// Vue d'ensemble - Statistiques globales
	adminRoutes.HandleFunc("/reports/overview", handlers.GetOverviewStatsHandler).Methods("GET")
	adminRoutes.HandleFunc("/reports/monthly", handlers.GetMonthlyStatsHandler).Methods("GET")
	adminRoutes.HandleFunc("/reports/categories", handlers.GetCategoryStatsHandler).Methods("GET")
	adminRoutes.HandleFunc("/reports/users", handlers.GetUserStatsHandler).Methods("GET")
	adminRoutes.HandleFunc("/reports/revenue", handlers.GetRevenueStatsHandler).Methods("GET")
	adminRoutes.HandleFunc("/reports/ads", handlers.GetAdStatsHandler).Methods("GET")
	adminRoutes.HandleFunc("/reports/custom", handlers.GetCustomReportHandler).Methods("POST")

	// ==============================================================
	// ROUTES ADMIN - PARAMÈTRES D'APPLICATION & MAINTENANCE
	// ==============================================================

	// Gestion des paramètres d'application
	adminRoutes.HandleFunc("/settings/app", handlers.GetAppSettingsForAdminHandler).Methods("GET")
	adminRoutes.HandleFunc("/settings/app", handlers.UpdateAppSettingsHandler).Methods("PUT")

	// Gestion du mode maintenance
	adminRoutes.HandleFunc("/maintenance", handlers.GetMaintenanceModeForAdminHandler).Methods("GET")
	adminRoutes.HandleFunc("/maintenance", handlers.UpdateMaintenanceModeHandler).Methods("PUT")

	return router
}
