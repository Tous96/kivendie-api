package handlers

import (
	"database/sql"
	"encoding/json"
	"kivendi-backend/config"
	"log"
	"net/http"
	"time"
)

// ============================================================================
// STRUCTURES POUR LES RAPPORTS
// ============================================================================

type OverviewStats struct {
	TotalUsers          int     `json:"total_users"`
	ActiveUsers         int     `json:"active_users"`
	NewUsersToday       int     `json:"new_users_today"`
	NewUsersThisWeek    int     `json:"new_users_this_week"`
	NewUsersThisMonth   int     `json:"new_users_this_month"`
	TotalAds            int     `json:"total_ads"`
	ValidatedAds        int     `json:"validated_ads"`
	PendingAds          int     `json:"pending_ads"`
	RejectedAds         int     `json:"rejected_ads"`
	DeactivatedAds      int     `json:"deactivated_ads"`
	SoldAds             int     `json:"sold_ads"`
	BoostedAds          int     `json:"boosted_ads"`
	TotalRevenue        float64 `json:"total_revenue"`
	RevenueThisMonth    float64 `json:"revenue_this_month"`
	TotalTransactions   int     `json:"total_transactions"`
	PendingReports      int     `json:"pending_reports"`
	PendingTickets      int     `json:"pending_tickets"`
	TotalConversations  int     `json:"total_conversations"`
	TotalMessages       int     `json:"total_messages"`
	TotalFavorites      int     `json:"total_favorites"`
	MostPopularCategory string  `json:"most_popular_category"`
	TotalCategories     int     `json:"total_categories"`
	TotalSubCategories  int     `json:"total_subcategories"`
	BlockedUsers        int     `json:"blocked_users"`
	ProUsers            int     `json:"pro_users"`
	PersonalUsers       int     `json:"personal_users"`
}

type MonthlyStats struct {
	Month               string  `json:"month"`
	Year                int     `json:"year"`
	NewUsers            int     `json:"new_users"`
	NewAds              int     `json:"new_ads"`
	ValidatedAds        int     `json:"validated_ads"`
	RejectedAds         int     `json:"rejected_ads"`
	SoldAds             int     `json:"sold_ads"`
	Revenue             float64 `json:"revenue"`
	Transactions        int     `json:"transactions"`
	NewConversations    int     `json:"new_conversations"`
	MessagesSent        int     `json:"messages_sent"`
	NewFavorites        int     `json:"new_favorites"`
	NewReports          int     `json:"new_reports"`
	NewTickets          int     `json:"new_tickets"`
	ActiveBoosts        int     `json:"active_boosts"`
	AverageAdPrice      float64 `json:"average_ad_price"`
	AverageResponseTime float64 `json:"average_response_time"`
}

type CategoryStats struct {
	CategoryID     int     `json:"category_id"`
	CategoryName   string  `json:"category_name"`
	TotalAds       int     `json:"total_ads"`
	ValidatedAds   int     `json:"validated_ads"`
	PendingAds     int     `json:"pending_ads"`
	SoldAds        int     `json:"sold_ads"`
	BoostedAds     int     `json:"boosted_ads"`
	AveragePrice   float64 `json:"average_price"`
	TotalViews     int     `json:"total_views"`
	TotalFavorites int     `json:"total_favorites"`
}

type UserStats struct {
	TotalUsers        int                 `json:"total_users"`
	ActiveUsers       int                 `json:"active_users"`
	InactiveUsers     int                 `json:"inactive_users"`
	VerifiedUsers     int                 `json:"verified_users"`
	UnverifiedUsers   int                 `json:"unverified_users"`
	BlockedUsers      int                 `json:"blocked_users"`
	ProUsers          int                 `json:"pro_users"`
	PersonalUsers     int                 `json:"personal_users"`
	UserGrowthByMonth []MonthlyUserGrowth `json:"user_growth_by_month"`
	TopSellers        []TopSeller         `json:"top_sellers"`
}

type MonthlyUserGrowth struct {
	Month    string `json:"month"`
	Year     int    `json:"year"`
	NewUsers int    `json:"new_users"`
	Total    int    `json:"total"`
}

type TopSeller struct {
	UserID       int     `json:"user_id"`
	DisplayName  string  `json:"display_name"`
	Email        string  `json:"email"`
	TotalAds     int     `json:"total_ads"`
	SoldAds      int     `json:"sold_ads"`
	TotalRevenue float64 `json:"total_revenue"`
	AvatarURL    string  `json:"avatar_url,omitempty"`
}

type RevenueStats struct {
	TotalRevenue          float64             `json:"total_revenue"`
	RevenueThisMonth      float64             `json:"revenue_this_month"`
	RevenueLastMonth      float64             `json:"revenue_last_month"`
	RevenueGrowth         float64             `json:"revenue_growth_percentage"`
	TotalTransactions     int                 `json:"total_transactions"`
	CompletedTransactions int                 `json:"completed_transactions"`
	PendingTransactions   int                 `json:"pending_transactions"`
	FailedTransactions    int                 `json:"failed_transactions"`
	AverageTransaction    float64             `json:"average_transaction"`
	RevenueByMonth        []MonthlyRevenue    `json:"revenue_by_month"`
	RevenueByBoostOffer   []BoostOfferRevenue `json:"revenue_by_boost_offer"`
}

type MonthlyRevenue struct {
	Month        string  `json:"month"`
	Year         int     `json:"year"`
	Revenue      float64 `json:"revenue"`
	Transactions int     `json:"transactions"`
}

type BoostOfferRevenue struct {
	OfferID    int     `json:"offer_id"`
	OfferName  string  `json:"offer_name"`
	TotalSales int     `json:"total_sales"`
	Revenue    float64 `json:"revenue"`
}

type AdStats struct {
	TotalAds           int              `json:"total_ads"`
	ValidatedAds       int              `json:"validated_ads"`
	PendingAds         int              `json:"pending_ads"`
	RejectedAds        int              `json:"rejected_ads"`
	DeactivatedAds     int              `json:"deactivated_ads"`
	SoldAds            int              `json:"sold_ads"`
	BoostedAds         int              `json:"boosted_ads"`
	AveragePrice       float64          `json:"average_price"`
	TotalViews         int              `json:"total_views"`
	AverageViews       float64          `json:"average_views"`
	AdsByMonth         []MonthlyAdStats `json:"ads_by_month"`
	AdsByCategory      []CategoryStats  `json:"ads_by_category"`
	TopViewedAds       []TopAd          `json:"top_viewed_ads"`
	RecentlyCreatedAds []TopAd          `json:"recently_created_ads"`
}

type MonthlyAdStats struct {
	Month        string  `json:"month"`
	Year         int     `json:"year"`
	NewAds       int     `json:"new_ads"`
	ValidatedAds int     `json:"validated_ads"`
	RejectedAds  int     `json:"rejected_ads"`
	SoldAds      int     `json:"sold_ads"`
	AveragePrice float64 `json:"average_price"`
}

type TopAd struct {
	AdID      int     `json:"ad_id"`
	Title     string  `json:"title"`
	Price     float64 `json:"price"`
	Views     int     `json:"views"`
	Category  string  `json:"category"`
	CreatedAt string  `json:"created_at"`
	ImageURL  string  `json:"image_url,omitempty"`
}

type CustomReportRequest struct {
	StartDate  string   `json:"start_date"`
	EndDate    string   `json:"end_date"`
	Metrics    []string `json:"metrics"`    // ex: ["users", "ads", "revenue"]
	GroupBy    string   `json:"group_by"`   // "day", "week", "month"
	Categories []int    `json:"categories"` // IDs des catégories à inclure
	UserTypes  []string `json:"user_types"` // ["Personnel", "Professionnel"]
}

type CustomReportResponse struct {
	Period      string                 `json:"period"`
	StartDate   string                 `json:"start_date"`
	EndDate     string                 `json:"end_date"`
	Data        map[string]interface{} `json:"data"`
	GeneratedAt string                 `json:"generated_at"`
}

// ============================================================================
// HANDLER: VUE D'ENSEMBLE
// ============================================================================

func GetOverviewStatsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	log.Println("📊 Récupération des statistiques d'ensemble...")

	var stats OverviewStats

	// Statistiques utilisateurs
	_ = config.DB.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&stats.TotalUsers)
	_ = config.DB.QueryRow(`SELECT COUNT(*) FROM users WHERE is_verified = true AND is_blocked = false`).Scan(&stats.ActiveUsers)
	_ = config.DB.QueryRow(`SELECT COUNT(*) FROM users WHERE created_at >= CURRENT_DATE`).Scan(&stats.NewUsersToday)
	_ = config.DB.QueryRow(`SELECT COUNT(*) FROM users WHERE created_at >= CURRENT_DATE - INTERVAL '7 days'`).Scan(&stats.NewUsersThisWeek)
	_ = config.DB.QueryRow(`SELECT COUNT(*) FROM users WHERE created_at >= DATE_TRUNC('month', CURRENT_DATE)`).Scan(&stats.NewUsersThisMonth)
	_ = config.DB.QueryRow(`SELECT COUNT(*) FROM users WHERE is_blocked = true`).Scan(&stats.BlockedUsers)
	_ = config.DB.QueryRow(`SELECT COUNT(*) FROM users WHERE account_type = 'Professionnel'`).Scan(&stats.ProUsers)
	_ = config.DB.QueryRow(`SELECT COUNT(*) FROM users WHERE account_type = 'Personnel'`).Scan(&stats.PersonalUsers)

	// Statistiques annonces
	_ = config.DB.QueryRow(`SELECT COUNT(*) FROM ads`).Scan(&stats.TotalAds)
	_ = config.DB.QueryRow(`SELECT COUNT(*) FROM ads WHERE is_validated = true`).Scan(&stats.ValidatedAds)
	_ = config.DB.QueryRow(`SELECT COUNT(*) FROM ads WHERE is_validated = false AND is_rejected = false AND is_deactivated = false`).Scan(&stats.PendingAds)
	_ = config.DB.QueryRow(`SELECT COUNT(*) FROM ads WHERE is_rejected = true`).Scan(&stats.RejectedAds)
	_ = config.DB.QueryRow(`SELECT COUNT(*) FROM ads WHERE is_deactivated = true`).Scan(&stats.DeactivatedAds)
	_ = config.DB.QueryRow(`SELECT COUNT(*) FROM ads WHERE is_sold = true`).Scan(&stats.SoldAds)
	_ = config.DB.QueryRow(`SELECT COUNT(*) FROM ads WHERE is_boosted = true`).Scan(&stats.BoostedAds)

	// Revenus
	_ = config.DB.QueryRow(`SELECT COALESCE(SUM(amount_paid), 0) FROM ad_boosts WHERE payment_status = 'completed'`).Scan(&stats.TotalRevenue)
	_ = config.DB.QueryRow(`
		SELECT COALESCE(SUM(amount_paid), 0) 
		FROM ad_boosts 
		WHERE payment_status = 'completed' 
		AND created_at >= DATE_TRUNC('month', CURRENT_DATE)
	`).Scan(&stats.RevenueThisMonth)
	_ = config.DB.QueryRow(`SELECT COUNT(*) FROM kkiapay_transactions`).Scan(&stats.TotalTransactions)

	// Signalements et tickets
	_ = config.DB.QueryRow(`SELECT COUNT(*) FROM user_reports WHERE status = 'pending'`).Scan(&stats.PendingReports)
	_ = config.DB.QueryRow(`SELECT COUNT(*) FROM support_tickets WHERE status = 'Nouveau'`).Scan(&stats.PendingTickets)

	// Communications
	_ = config.DB.QueryRow(`SELECT COUNT(*) FROM conversations`).Scan(&stats.TotalConversations)
	_ = config.DB.QueryRow(`SELECT COUNT(*) FROM messages`).Scan(&stats.TotalMessages)
	_ = config.DB.QueryRow(`SELECT COUNT(*) FROM favorites`).Scan(&stats.TotalFavorites)

	// Catégories
	_ = config.DB.QueryRow(`SELECT COUNT(*) FROM categories`).Scan(&stats.TotalCategories)
	_ = config.DB.QueryRow(`SELECT COUNT(*) FROM sub_categories`).Scan(&stats.TotalSubCategories)

	// Catégorie la plus populaire
	_ = config.DB.QueryRow(`
		SELECT c.name 
		FROM categories c
		JOIN sub_categories sc ON c.id = sc.category_id
		JOIN ads a ON sc.id = a.sub_category_id
		GROUP BY c.id, c.name
		ORDER BY COUNT(a.id) DESC
		LIMIT 1
	`).Scan(&stats.MostPopularCategory)

	log.Println("✅ Statistiques d'ensemble récupérées avec succès")
	json.NewEncoder(w).Encode(stats)
}

// ============================================================================
// HANDLER: STATISTIQUES MENSUELLES
// ============================================================================

func GetMonthlyStatsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	log.Println("📊 Récupération des statistiques mensuelles...")

	// Récupérer les 12 derniers mois
	query := `
		WITH months AS (
			SELECT 
				TO_CHAR(DATE_TRUNC('month', CURRENT_DATE - (n || ' months')::INTERVAL), 'YYYY-MM') as month_key,
				TO_CHAR(DATE_TRUNC('month', CURRENT_DATE - (n || ' months')::INTERVAL), 'Month') as month_name,
				EXTRACT(YEAR FROM DATE_TRUNC('month', CURRENT_DATE - (n || ' months')::INTERVAL))::INT as year
			FROM generate_series(0, 11) n
		)
		SELECT 
			m.month_name,
			m.year,
			COALESCE(COUNT(DISTINCT u.id), 0) as new_users,
			COALESCE(COUNT(DISTINCT a.id), 0) as new_ads,
			COALESCE(COUNT(DISTINCT CASE WHEN a.is_validated = true THEN a.id END), 0) as validated_ads,
			COALESCE(COUNT(DISTINCT CASE WHEN a.is_rejected = true THEN a.id END), 0) as rejected_ads,
			COALESCE(COUNT(DISTINCT CASE WHEN a.is_sold = true THEN a.id END), 0) as sold_ads,
			COALESCE(SUM(ab.amount_paid), 0) as revenue,
			COALESCE(COUNT(DISTINCT ab.id), 0) as transactions,
			COALESCE(COUNT(DISTINCT c.id), 0) as new_conversations,
			COALESCE(COUNT(DISTINCT msg.id), 0) as messages_sent,
			COALESCE(COUNT(DISTINCT f.ad_id), 0) as new_favorites,
			COALESCE(COUNT(DISTINCT ur.id), 0) as new_reports,
			COALESCE(COUNT(DISTINCT st.id), 0) as new_tickets,
			COALESCE(COUNT(DISTINCT CASE WHEN ab.is_active = true THEN ab.id END), 0) as active_boosts,
			COALESCE(AVG(a.price), 0) as average_ad_price
		FROM months m
		LEFT JOIN users u ON TO_CHAR(DATE_TRUNC('month', u.created_at), 'YYYY-MM') = m.month_key
		LEFT JOIN ads a ON TO_CHAR(DATE_TRUNC('month', a.created_at), 'YYYY-MM') = m.month_key
		LEFT JOIN ad_boosts ab ON TO_CHAR(DATE_TRUNC('month', ab.created_at), 'YYYY-MM') = m.month_key 
			AND ab.payment_status = 'completed'
		LEFT JOIN conversations c ON TO_CHAR(DATE_TRUNC('month', c.created_at), 'YYYY-MM') = m.month_key
		LEFT JOIN messages msg ON TO_CHAR(DATE_TRUNC('month', msg.created_at), 'YYYY-MM') = m.month_key
		LEFT JOIN favorites f ON TO_CHAR(DATE_TRUNC('month', f.created_at), 'YYYY-MM') = m.month_key
		LEFT JOIN user_reports ur ON TO_CHAR(DATE_TRUNC('month', ur.created_at), 'YYYY-MM') = m.month_key
		LEFT JOIN support_tickets st ON TO_CHAR(DATE_TRUNC('month', st.created_at), 'YYYY-MM') = m.month_key
		GROUP BY m.month_key, m.month_name, m.year
		ORDER BY m.month_key DESC
	`

	rows, err := config.DB.Query(query)
	if err != nil {
		log.Printf("❌ Erreur lors de la récupération des stats mensuelles: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var monthlyStats []MonthlyStats
	for rows.Next() {
		var stat MonthlyStats
		err := rows.Scan(
			&stat.Month,
			&stat.Year,
			&stat.NewUsers,
			&stat.NewAds,
			&stat.ValidatedAds,
			&stat.RejectedAds,
			&stat.SoldAds,
			&stat.Revenue,
			&stat.Transactions,
			&stat.NewConversations,
			&stat.MessagesSent,
			&stat.NewFavorites,
			&stat.NewReports,
			&stat.NewTickets,
			&stat.ActiveBoosts,
			&stat.AverageAdPrice,
		)
		if err != nil {
			log.Printf("❌ Erreur lors du scan: %v", err)
			continue
		}
		monthlyStats = append(monthlyStats, stat)
	}

	log.Println("✅ Statistiques mensuelles récupérées avec succès")
	json.NewEncoder(w).Encode(monthlyStats)
}

// ============================================================================
// HANDLER: STATISTIQUES PAR CATÉGORIE
// ============================================================================

func GetCategoryStatsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	log.Println("📊 Récupération des statistiques par catégorie...")

	query := `
		SELECT 
			c.id,
			c.name,
			COUNT(DISTINCT a.id) as total_ads,
			COUNT(DISTINCT CASE WHEN a.is_validated = true THEN a.id END) as validated_ads,
			COUNT(DISTINCT CASE WHEN a.is_validated = false AND a.is_rejected = false AND a.is_deactivated = false THEN a.id END) as pending_ads,
			COUNT(DISTINCT CASE WHEN a.is_sold = true THEN a.id END) as sold_ads,
			COUNT(DISTINCT CASE WHEN a.is_boosted = true THEN a.id END) as boosted_ads,
			COALESCE(AVG(a.price), 0) as average_price,
			COALESCE(SUM(a.views_count), 0) as total_views,
			COUNT(DISTINCT f.user_id) as total_favorites
		FROM categories c
		LEFT JOIN sub_categories sc ON c.id = sc.category_id
		LEFT JOIN ads a ON sc.id = a.sub_category_id
		LEFT JOIN favorites f ON a.id = f.ad_id
		GROUP BY c.id, c.name
		ORDER BY total_ads DESC
	`

	rows, err := config.DB.Query(query)
	if err != nil {
		log.Printf("❌ Erreur lors de la récupération des stats par catégorie: %v", err)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var categoryStats []CategoryStats
	for rows.Next() {
		var stat CategoryStats
		err := rows.Scan(
			&stat.CategoryID,
			&stat.CategoryName,
			&stat.TotalAds,
			&stat.ValidatedAds,
			&stat.PendingAds,
			&stat.SoldAds,
			&stat.BoostedAds,
			&stat.AveragePrice,
			&stat.TotalViews,
			&stat.TotalFavorites,
		)
		if err != nil {
			log.Printf("❌ Erreur lors du scan: %v", err)
			continue
		}
		categoryStats = append(categoryStats, stat)
	}

	log.Println("✅ Statistiques par catégorie récupérées avec succès")
	json.NewEncoder(w).Encode(categoryStats)
}

// ============================================================================
// HANDLER: STATISTIQUES UTILISATEURS
// ============================================================================

func GetUserStatsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	log.Println("📊 Récupération des statistiques utilisateurs...")

	var stats UserStats

	// Statistiques générales
	_ = config.DB.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&stats.TotalUsers)
	_ = config.DB.QueryRow(`
		SELECT COUNT(*) FROM users 
		WHERE is_verified = true 
		AND is_blocked = false
	`).Scan(&stats.ActiveUsers)
	_ = config.DB.QueryRow(`
		SELECT COUNT(*) FROM users 
		WHERE is_verified = false OR is_blocked = true
	`).Scan(&stats.InactiveUsers)
	_ = config.DB.QueryRow(`SELECT COUNT(*) FROM users WHERE is_verified = true`).Scan(&stats.VerifiedUsers)
	_ = config.DB.QueryRow(`SELECT COUNT(*) FROM users WHERE is_verified = false`).Scan(&stats.UnverifiedUsers)
	_ = config.DB.QueryRow(`SELECT COUNT(*) FROM users WHERE is_blocked = true`).Scan(&stats.BlockedUsers)
	_ = config.DB.QueryRow(`SELECT COUNT(*) FROM users WHERE account_type = 'Professionnel'`).Scan(&stats.ProUsers)
	_ = config.DB.QueryRow(`SELECT COUNT(*) FROM users WHERE account_type = 'Personnel'`).Scan(&stats.PersonalUsers)

	// Croissance mensuelle
	growthQuery := `
		WITH months AS (
			SELECT 
				TO_CHAR(DATE_TRUNC('month', CURRENT_DATE - (n || ' months')::INTERVAL), 'Month') as month_name,
				EXTRACT(YEAR FROM DATE_TRUNC('month', CURRENT_DATE - (n || ' months')::INTERVAL))::INT as year,
				DATE_TRUNC('month', CURRENT_DATE - (n || ' months')::INTERVAL) as month_date
			FROM generate_series(0, 11) n
		)
		SELECT 
			m.month_name,
			m.year,
			COUNT(u.id) as new_users,
			(SELECT COUNT(*) FROM users WHERE created_at <= m.month_date + INTERVAL '1 month' - INTERVAL '1 day') as total
		FROM months m
		LEFT JOIN users u ON DATE_TRUNC('month', u.created_at) = m.month_date
		GROUP BY m.month_name, m.year, m.month_date
		ORDER BY m.month_date DESC
	`

	rows, err := config.DB.Query(growthQuery)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var growth MonthlyUserGrowth
			rows.Scan(&growth.Month, &growth.Year, &growth.NewUsers, &growth.Total)
			stats.UserGrowthByMonth = append(stats.UserGrowthByMonth, growth)
		}
	}

	// Top vendeurs
	topSellersQuery := `
		SELECT 
			u.id,
			CASE 
				WHEN u.account_type = 'Professionnel' AND u.shop_name IS NOT NULL 
				THEN u.shop_name 
				ELSE u.first_name || ' ' || u.last_name 
			END as display_name,
			u.email,
			u.avatar_url,
			COUNT(DISTINCT a.id) as total_ads,
			COUNT(DISTINCT CASE WHEN a.is_sold = true THEN a.id END) as sold_ads,
			COALESCE(SUM(CASE WHEN a.is_sold = true THEN a.price END), 0) as total_revenue
		FROM users u
		LEFT JOIN ads a ON u.id = a.user_id
		GROUP BY u.id, display_name, u.email, u.avatar_url
		HAVING COUNT(DISTINCT a.id) > 0
		ORDER BY sold_ads DESC, total_ads DESC
		LIMIT 10
	`

	rows2, err := config.DB.Query(topSellersQuery)
	if err == nil {
		defer rows2.Close()
		for rows2.Next() {
			var seller TopSeller
			var avatarURL sql.NullString
			rows2.Scan(
				&seller.UserID,
				&seller.DisplayName,
				&seller.Email,
				&avatarURL,
				&seller.TotalAds,
				&seller.SoldAds,
				&seller.TotalRevenue,
			)
			if avatarURL.Valid {
				seller.AvatarURL = avatarURL.String
			}
			stats.TopSellers = append(stats.TopSellers, seller)
		}
	}

	log.Println("✅ Statistiques utilisateurs récupérées avec succès")
	json.NewEncoder(w).Encode(stats)
}

// ============================================================================
// HANDLER: STATISTIQUES REVENUS
// ============================================================================

func GetRevenueStatsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	log.Println("📊 Récupération des statistiques de revenus...")

	var stats RevenueStats

	// Revenus globaux
	_ = config.DB.QueryRow(`
		SELECT COALESCE(SUM(amount_paid), 0) 
		FROM ad_boosts 
		WHERE payment_status = 'completed'
	`).Scan(&stats.TotalRevenue)

	_ = config.DB.QueryRow(`
		SELECT COALESCE(SUM(amount_paid), 0) 
		FROM ad_boosts 
		WHERE payment_status = 'completed' 
		AND created_at >= DATE_TRUNC('month', CURRENT_DATE)
	`).Scan(&stats.RevenueThisMonth)

	_ = config.DB.QueryRow(`
		SELECT COALESCE(SUM(amount_paid), 0) 
		FROM ad_boosts 
		WHERE payment_status = 'completed' 
		AND created_at >= DATE_TRUNC('month', CURRENT_DATE) - INTERVAL '1 month'
		AND created_at < DATE_TRUNC('month', CURRENT_DATE)
	`).Scan(&stats.RevenueLastMonth)

	// Calcul de la croissance
	if stats.RevenueLastMonth > 0 {
		stats.RevenueGrowth = ((stats.RevenueThisMonth - stats.RevenueLastMonth) / stats.RevenueLastMonth) * 100
	}

	// Transactions
	_ = config.DB.QueryRow(`SELECT COUNT(*) FROM kkiapay_transactions`).Scan(&stats.TotalTransactions)
	_ = config.DB.QueryRow(`
		SELECT COUNT(*) FROM ad_boosts WHERE payment_status = 'completed'
	`).Scan(&stats.CompletedTransactions)
	_ = config.DB.QueryRow(`
		SELECT COUNT(*) FROM ad_boosts WHERE payment_status = 'pending'
	`).Scan(&stats.PendingTransactions)
	_ = config.DB.QueryRow(`
		SELECT COUNT(*) FROM ad_boosts WHERE payment_status = 'failed'
	`).Scan(&stats.FailedTransactions)

	// Moyenne par transaction
	if stats.CompletedTransactions > 0 {
		stats.AverageTransaction = stats.TotalRevenue / float64(stats.CompletedTransactions)
	}

	// Revenus mensuels
	monthlyRevenueQuery := `
		WITH months AS (
			SELECT 
				TO_CHAR(DATE_TRUNC('month', CURRENT_DATE - (n || ' months')::INTERVAL), 'Month') as month_name,
				EXTRACT(YEAR FROM DATE_TRUNC('month', CURRENT_DATE - (n || ' months')::INTERVAL))::INT as year,
				DATE_TRUNC('month', CURRENT_DATE - (n || ' months')::INTERVAL) as month_date
			FROM generate_series(0, 11) n
		)
		SELECT 
			m.month_name,
			m.year,
			COALESCE(SUM(ab.amount_paid), 0) as revenue,
			COUNT(ab.id) as transactions
		FROM months m
		LEFT JOIN ad_boosts ab ON DATE_TRUNC('month', ab.created_at) = m.month_date 
			AND ab.payment_status = 'completed'
		GROUP BY m.month_name, m.year, m.month_date
		ORDER BY m.month_date DESC
	`

	rows, err := config.DB.Query(monthlyRevenueQuery)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var rev MonthlyRevenue
			rows.Scan(&rev.Month, &rev.Year, &rev.Revenue, &rev.Transactions)
			stats.RevenueByMonth = append(stats.RevenueByMonth, rev)
		}
	}

	// Revenus par offre de boost
	offerRevenueQuery := `
		SELECT 
			bo.id,
			bo.name,
			COUNT(ab.id) as total_sales,
			COALESCE(SUM(ab.amount_paid), 0) as revenue
		FROM boost_offers bo
		LEFT JOIN ad_boosts ab ON bo.id = ab.boost_offer_id 
			AND ab.payment_status = 'completed'
		GROUP BY bo.id, bo.name
		ORDER BY revenue DESC
	`

	rows2, err := config.DB.Query(offerRevenueQuery)
	if err == nil {
		defer rows2.Close()
		for rows2.Next() {
			var offerRev BoostOfferRevenue
			rows2.Scan(&offerRev.OfferID, &offerRev.OfferName, &offerRev.TotalSales, &offerRev.Revenue)
			stats.RevenueByBoostOffer = append(stats.RevenueByBoostOffer, offerRev)
		}
	}

	log.Println("✅ Statistiques de revenus récupérées avec succès")
	json.NewEncoder(w).Encode(stats)
}

// ============================================================================
// HANDLER: STATISTIQUES ANNONCES
// ============================================================================

func GetAdStatsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	log.Println("📊 Récupération des statistiques des annonces...")

	var stats AdStats

	// Statistiques générales
	_ = config.DB.QueryRow(`SELECT COUNT(*) FROM ads`).Scan(&stats.TotalAds)
	_ = config.DB.QueryRow(`SELECT COUNT(*) FROM ads WHERE is_validated = true`).Scan(&stats.ValidatedAds)
	_ = config.DB.QueryRow(`
		SELECT COUNT(*) FROM ads 
		WHERE is_validated = false AND is_rejected = false AND is_deactivated = false
	`).Scan(&stats.PendingAds)
	_ = config.DB.QueryRow(`SELECT COUNT(*) FROM ads WHERE is_rejected = true`).Scan(&stats.RejectedAds)
	_ = config.DB.QueryRow(`SELECT COUNT(*) FROM ads WHERE is_deactivated = true`).Scan(&stats.DeactivatedAds)
	_ = config.DB.QueryRow(`SELECT COUNT(*) FROM ads WHERE is_sold = true`).Scan(&stats.SoldAds)
	_ = config.DB.QueryRow(`SELECT COUNT(*) FROM ads WHERE is_boosted = true`).Scan(&stats.BoostedAds)
	_ = config.DB.QueryRow(`SELECT COALESCE(AVG(price), 0) FROM ads`).Scan(&stats.AveragePrice)
	_ = config.DB.QueryRow(`SELECT COALESCE(SUM(views_count), 0) FROM ads`).Scan(&stats.TotalViews)

	if stats.TotalAds > 0 {
		stats.AverageViews = float64(stats.TotalViews) / float64(stats.TotalAds)
	}

	// Annonces par mois
	monthlyAdsQuery := `
		WITH months AS (
			SELECT 
				TO_CHAR(DATE_TRUNC('month', CURRENT_DATE - (n || ' months')::INTERVAL), 'Month') as month_name,
				EXTRACT(YEAR FROM DATE_TRUNC('month', CURRENT_DATE - (n || ' months')::INTERVAL))::INT as year,
				DATE_TRUNC('month', CURRENT_DATE - (n || ' months')::INTERVAL) as month_date
			FROM generate_series(0, 11) n
		)
		SELECT 
			m.month_name,
			m.year,
			COUNT(a.id) as new_ads,
			COUNT(CASE WHEN a.is_validated = true THEN 1 END) as validated_ads,
			COUNT(CASE WHEN a.is_rejected = true THEN 1 END) as rejected_ads,
			COUNT(CASE WHEN a.is_sold = true THEN 1 END) as sold_ads,
			COALESCE(AVG(a.price), 0) as average_price
		FROM months m
		LEFT JOIN ads a ON DATE_TRUNC('month', a.created_at) = m.month_date
		GROUP BY m.month_name, m.year, m.month_date
		ORDER BY m.month_date DESC
	`

	rows, err := config.DB.Query(monthlyAdsQuery)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var monthly MonthlyAdStats
			rows.Scan(
				&monthly.Month,
				&monthly.Year,
				&monthly.NewAds,
				&monthly.ValidatedAds,
				&monthly.RejectedAds,
				&monthly.SoldAds,
				&monthly.AveragePrice,
			)
			stats.AdsByMonth = append(stats.AdsByMonth, monthly)
		}
	}

	// Annonces par catégorie (réutilisation de la logique)
	categoryQuery := `
		SELECT 
			c.id,
			c.name,
			COUNT(DISTINCT a.id) as total_ads,
			COUNT(DISTINCT CASE WHEN a.is_validated = true THEN a.id END) as validated_ads,
			COUNT(DISTINCT CASE WHEN a.is_validated = false AND a.is_rejected = false AND a.is_deactivated = false THEN a.id END) as pending_ads,
			COUNT(DISTINCT CASE WHEN a.is_sold = true THEN a.id END) as sold_ads,
			COUNT(DISTINCT CASE WHEN a.is_boosted = true THEN a.id END) as boosted_ads,
			COALESCE(AVG(a.price), 0) as average_price,
			COALESCE(SUM(a.views_count), 0) as total_views,
			COUNT(DISTINCT f.user_id) as total_favorites
		FROM categories c
		LEFT JOIN sub_categories sc ON c.id = sc.category_id
		LEFT JOIN ads a ON sc.id = a.sub_category_id
		LEFT JOIN favorites f ON a.id = f.ad_id
		GROUP BY c.id, c.name
		ORDER BY total_ads DESC
	`

	rows2, err := config.DB.Query(categoryQuery)
	if err == nil {
		defer rows2.Close()
		for rows2.Next() {
			var cat CategoryStats
			rows2.Scan(
				&cat.CategoryID,
				&cat.CategoryName,
				&cat.TotalAds,
				&cat.ValidatedAds,
				&cat.PendingAds,
				&cat.SoldAds,
				&cat.BoostedAds,
				&cat.AveragePrice,
				&cat.TotalViews,
				&cat.TotalFavorites,
			)
			stats.AdsByCategory = append(stats.AdsByCategory, cat)
		}
	}

	// Top annonces les plus vues
	topViewedQuery := `
		SELECT 
			a.id,
			a.title,
			a.price,
			a.views_count,
			c.name as category,
			TO_CHAR(a.created_at, 'YYYY-MM-DD') as created_at,
			a.images[1] as image_url
		FROM ads a
		JOIN sub_categories sc ON a.sub_category_id = sc.id
		JOIN categories c ON sc.category_id = c.id
		WHERE a.is_validated = true
		ORDER BY a.views_count DESC
		LIMIT 10
	`

	rows3, err := config.DB.Query(topViewedQuery)
	if err == nil {
		defer rows3.Close()
		for rows3.Next() {
			var ad TopAd
			var imageURL sql.NullString
			rows3.Scan(&ad.AdID, &ad.Title, &ad.Price, &ad.Views, &ad.Category, &ad.CreatedAt, &imageURL)
			if imageURL.Valid {
				ad.ImageURL = imageURL.String
			}
			stats.TopViewedAds = append(stats.TopViewedAds, ad)
		}
	}

	// Annonces récemment créées
	recentAdsQuery := `
		SELECT 
			a.id,
			a.title,
			a.price,
			a.views_count,
			c.name as category,
			TO_CHAR(a.created_at, 'YYYY-MM-DD HH24:MI') as created_at,
			a.images[1] as image_url
		FROM ads a
		JOIN sub_categories sc ON a.sub_category_id = sc.id
		JOIN categories c ON sc.category_id = c.id
		ORDER BY a.created_at DESC
		LIMIT 10
	`

	rows4, err := config.DB.Query(recentAdsQuery)
	if err == nil {
		defer rows4.Close()
		for rows4.Next() {
			var ad TopAd
			var imageURL sql.NullString
			rows4.Scan(&ad.AdID, &ad.Title, &ad.Price, &ad.Views, &ad.Category, &ad.CreatedAt, &imageURL)
			if imageURL.Valid {
				ad.ImageURL = imageURL.String
			}
			stats.RecentlyCreatedAds = append(stats.RecentlyCreatedAds, ad)
		}
	}

	log.Println("✅ Statistiques des annonces récupérées avec succès")
	json.NewEncoder(w).Encode(stats)
}

// ============================================================================
// HANDLER: RAPPORT PERSONNALISÉ
// ============================================================================

func GetCustomReportHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	log.Println("📊 Génération d'un rapport personnalisé...")

	var req CustomReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Données invalides", http.StatusBadRequest)
		return
	}

	// Validation des dates
	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		http.Error(w, "Date de début invalide", http.StatusBadRequest)
		return
	}

	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		http.Error(w, "Date de fin invalide", http.StatusBadRequest)
		return
	}

	response := CustomReportResponse{
		StartDate:   req.StartDate,
		EndDate:     req.EndDate,
		GeneratedAt: time.Now().Format("2006-01-02 15:04:05"),
		Data:        make(map[string]interface{}),
	}

	// Générer le rapport en fonction des métriques demandées
	for _, metric := range req.Metrics {
		switch metric {
		case "users":
			var count int
			config.DB.QueryRow(`
				SELECT COUNT(*) FROM users 
				WHERE created_at BETWEEN $1 AND $2
			`, startDate, endDate).Scan(&count)
			response.Data["new_users"] = count

		case "ads":
			var count int
			config.DB.QueryRow(`
				SELECT COUNT(*) FROM ads 
				WHERE created_at BETWEEN $1 AND $2
			`, startDate, endDate).Scan(&count)
			response.Data["new_ads"] = count

		case "revenue":
			var revenue float64
			config.DB.QueryRow(`
				SELECT COALESCE(SUM(amount_paid), 0) 
				FROM ad_boosts 
				WHERE payment_status = 'completed'
				AND created_at BETWEEN $1 AND $2
			`, startDate, endDate).Scan(&revenue)
			response.Data["revenue"] = revenue

		case "conversations":
			var count int
			config.DB.QueryRow(`
				SELECT COUNT(*) FROM conversations 
				WHERE created_at BETWEEN $1 AND $2
			`, startDate, endDate).Scan(&count)
			response.Data["conversations"] = count

		case "messages":
			var count int
			config.DB.QueryRow(`
				SELECT COUNT(*) FROM messages 
				WHERE created_at BETWEEN $1 AND $2
			`, startDate, endDate).Scan(&count)
			response.Data["messages"] = count
		}
	}

	response.Period = req.GroupBy
	log.Println("✅ Rapport personnalisé généré avec succès")
	json.NewEncoder(w).Encode(response)
}
