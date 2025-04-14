package models

import (
	"database/sql"
	"errors"
	"forum/internal/database"
	"time"
)

// Report représente un signalement de contenu inapproprié
type Report struct {
	ID            int
	Type          string    // "post" ou "comment"
	ContentID     int       // ID du post ou du commentaire
	ReporterID    int       // Utilisateur qui a fait le signalement
	ReporterName  string    // Nom d'utilisateur du reporter
	Reason        string    // Raison du signalement
	Status        string    // "pending", "approved", "rejected"
	AdminID       sql.NullInt64   // Admin qui a traité le signalement
	AdminResponse sql.NullString  // Réponse de l'administrateur
	CreatedAt     time.Time // Date de création du signalement
	UpdatedAt     time.Time // Date de la dernière mise à jour
}

// PendingPost représente un post en attente de modération
type PendingPost struct {
	ID           int
	Title        string
	Content      string
	UserID       int
	Username     string
	Categories   []Category
	Status       string    // "pending", "approved", "rejected"
	ModeratorID  sql.NullInt64
	Reason       sql.NullString
	CreatedAt    time.Time
}

// ModerationQueueInit initialise les tables nécessaires pour la modération
func ModerationQueueInit() error {
	// Création de la table pour les posts en attente de modération
	pendingPostsTable := `
	CREATE TABLE IF NOT EXISTS pending_posts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		content TEXT NOT NULL,
		user_id INTEGER NOT NULL,
		status TEXT DEFAULT 'pending',
		moderator_id INTEGER,
		reason TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (user_id) REFERENCES users(id),
		FOREIGN KEY (moderator_id) REFERENCES users(id)
	);`
	_, err := database.DB.Exec(pendingPostsTable)
	if err != nil {
		return err
	}

	// Création de la table pour les catégories des posts en attente
	pendingPostCategoriesTable := `
	CREATE TABLE IF NOT EXISTS pending_post_categories (
		post_id INTEGER NOT NULL,
		category_id INTEGER NOT NULL,
		PRIMARY KEY (post_id, category_id),
		FOREIGN KEY (post_id) REFERENCES pending_posts(id) ON DELETE CASCADE,
		FOREIGN KEY (category_id) REFERENCES categories(id) ON DELETE CASCADE
	);`
	_, err = database.DB.Exec(pendingPostCategoriesTable)
	if err != nil {
		return err
	}

	// Création de la table pour les signalements
	reportsTable := `
	CREATE TABLE IF NOT EXISTS reports (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		type TEXT NOT NULL,
		content_id INTEGER NOT NULL,
		reporter_id INTEGER NOT NULL,
		reason TEXT NOT NULL,
		status TEXT DEFAULT 'pending',
		admin_id INTEGER,
		admin_response TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (reporter_id) REFERENCES users(id),
		FOREIGN KEY (admin_id) REFERENCES users(id)
	);`
	_, err = database.DB.Exec(reportsTable)
	if err != nil {
		return err
	}

	// Modification de la table users pour ajouter la colonne role si elle n'existe pas déjà
	// C'est déjà fait dans la structure initiale.

	return nil
}

// SubmitPendingPost soumet un post à la modération
func SubmitPendingPost(title, content string, userID int, categoryIDs []int) (int, error) {
	// Vérifier si le titre et le contenu ne sont pas vides
	if title == "" || content == "" {
		return 0, errors.New("title and content are required")
	}

	// Démarrer une transaction
	tx, err := database.DB.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	// Insérer le post dans la table des posts en attente
	result, err := tx.Exec(
		"INSERT INTO pending_posts (title, content, user_id, status) VALUES (?, ?, ?, ?)",
		title, content, userID, "pending",
	)
	if err != nil {
		return 0, err
	}

	// Récupérer l'ID du post nouvellement créé
	postID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	// Associer les catégories au post en attente
	for _, categoryID := range categoryIDs {
		_, err = tx.Exec(
			"INSERT INTO pending_post_categories (post_id, category_id) VALUES (?, ?)",
			postID, categoryID,
		)
		if err != nil {
			return 0, err
		}
	}

	// Valider la transaction
	err = tx.Commit()
	if err != nil {
		return 0, err
	}

	return int(postID), nil
}

// GetPendingPosts récupère les posts en attente de modération
func GetPendingPosts() ([]*PendingPost, error) {
	rows, err := database.DB.Query(`
		SELECT p.id, p.title, p.content, p.user_id, u.username, p.status, p.moderator_id, p.reason, p.created_at
		FROM pending_posts p
		JOIN users u ON p.user_id = u.id
		WHERE p.status = 'pending'
		ORDER BY p.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pendingPosts []*PendingPost
	for rows.Next() {
		post := &PendingPost{}
		err = rows.Scan(
			&post.ID, &post.Title, &post.Content, &post.UserID, &post.Username,
			&post.Status, &post.ModeratorID, &post.Reason, &post.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Récupérer les catégories pour ce post
		categoryRows, err := database.DB.Query(`
			SELECT c.id, c.name, c.description
			FROM categories c
			JOIN pending_post_categories pc ON c.id = pc.category_id
			WHERE pc.post_id = ?
		`, post.ID)
		if err != nil {
			return nil, err
		}

		post.Categories = []Category{}
		for categoryRows.Next() {
			var category Category
			err = categoryRows.Scan(&category.ID, &category.Name, &category.Description)
			if err != nil {
				categoryRows.Close()
				return nil, err
			}
			post.Categories = append(post.Categories, category)
		}
		categoryRows.Close()

		pendingPosts = append(pendingPosts, post)
	}

	return pendingPosts, nil
}

// ApprovePendingPost approuve un post en attente et le publie
func ApprovePendingPost(pendingID, moderatorID int) (int, error) {
	// Récupérer les informations du post en attente
	pendingPost := &PendingPost{}
	err := database.DB.QueryRow(`
		SELECT id, title, content, user_id, status FROM pending_posts
		WHERE id = ? AND status = 'pending'
	`, pendingID).Scan(
		&pendingPost.ID, &pendingPost.Title, &pendingPost.Content, &pendingPost.UserID, &pendingPost.Status,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, errors.New("pending post not found or already processed")
		}
		return 0, err
	}

	// Récupérer les catégories associées au post en attente
	rows, err := database.DB.Query(`
		SELECT category_id FROM pending_post_categories
		WHERE post_id = ?
	`, pendingID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var categoryIDs []int
	for rows.Next() {
		var categoryID int
		err = rows.Scan(&categoryID)
		if err != nil {
			return 0, err
		}
		categoryIDs = append(categoryIDs, categoryID)
	}

	// Démarrer une transaction
	tx, err := database.DB.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	// Créer le post approuvé
	result, err := tx.Exec(
		"INSERT INTO posts (title, content, user_id) VALUES (?, ?, ?)",
		pendingPost.Title, pendingPost.Content, pendingPost.UserID,
	)
	if err != nil {
		return 0, err
	}

	postID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	// Associer les catégories au nouveau post
	for _, categoryID := range categoryIDs {
		_, err = tx.Exec(
			"INSERT INTO post_categories (post_id, category_id) VALUES (?, ?)",
			postID, categoryID,
		)
		if err != nil {
			return 0, err
		}
	}

	// Mettre à jour le statut du post en attente
	_, err = tx.Exec(
		"UPDATE pending_posts SET status = 'approved', moderator_id = ? WHERE id = ?",
		moderatorID, pendingID,
	)
	if err != nil {
		return 0, err
	}

	// Valider la transaction
	err = tx.Commit()
	if err != nil {
		return 0, err
	}

	return int(postID), nil
}

// RejectPendingPost rejette un post en attente
func RejectPendingPost(pendingID, moderatorID int, reason string) error {
	// Vérifier si le post en attente existe et est en attente
	var status string
	err := database.DB.QueryRow(
		"SELECT status FROM pending_posts WHERE id = ?",
		pendingID,
	).Scan(&status)
	if err != nil {
		if err == sql.ErrNoRows {
			return errors.New("pending post not found")
		}
		return err
	}
	if status != "pending" {
		return errors.New("post is not in pending status")
	}

	// Mettre à jour le statut du post en attente
	_, err = database.DB.Exec(
		"UPDATE pending_posts SET status = 'rejected', moderator_id = ?, reason = ? WHERE id = ?",
		moderatorID, reason, pendingID,
	)
	if err != nil {
		return err
	}

	return nil
}

// ReportContent signale un contenu inapproprié
func ReportContent(contentType string, contentID, reporterID int, reason string) (int, error) {
	// Vérifier que le type de contenu est valide
	if contentType != "post" && contentType != "comment" {
		return 0, errors.New("invalid content type")
	}

	// Vérifier que le contenu existe
	var count int
	var query string
	if contentType == "post" {
		query = "SELECT COUNT(*) FROM posts WHERE id = ?"
	} else {
		query = "SELECT COUNT(*) FROM comments WHERE id = ?"
	}
	err := database.DB.QueryRow(query, contentID).Scan(&count)
	if err != nil {
		return 0, err
	}
	if count == 0 {
		return 0, errors.New("content not found")
	}

	// Vérifier si l'utilisateur a déjà signalé ce contenu
	err = database.DB.QueryRow(
		"SELECT COUNT(*) FROM reports WHERE type = ? AND content_id = ? AND reporter_id = ?",
		contentType, contentID, reporterID,
	).Scan(&count)
	if err != nil {
		return 0, err
	}
	if count > 0 {
		return 0, errors.New("content already reported by this user")
	}

	// Créer le signalement
	result, err := database.DB.Exec(
		"INSERT INTO reports (type, content_id, reporter_id, reason, status) VALUES (?, ?, ?, ?, ?)",
		contentType, contentID, reporterID, reason, "pending",
	)
	if err != nil {
		return 0, err
	}

	reportID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return int(reportID), nil
}

// GetReports récupère les signalements en attente
func GetReports() ([]*Report, error) {
	rows, err := database.DB.Query(`
		SELECT r.id, r.type, r.content_id, r.reporter_id, u.username, r.reason, 
		       r.status, r.admin_id, r.admin_response, r.created_at, r.updated_at
		FROM reports r
		JOIN users u ON r.reporter_id = u.id
		WHERE r.status = 'pending'
		ORDER BY r.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reports []*Report
	for rows.Next() {
		report := &Report{}
		err = rows.Scan(
			&report.ID, &report.Type, &report.ContentID, &report.ReporterID, &report.ReporterName,
			&report.Reason, &report.Status, &report.AdminID, &report.AdminResponse,
			&report.CreatedAt, &report.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		reports = append(reports, report)
	}

	return reports, nil
}

// HandleReport traite un signalement (approuver ou rejeter)
func HandleReport(reportID, adminID int, status string, response string) error {
	// Vérifier que le statut est valide
	if status != "approved" && status != "rejected" {
		return errors.New("invalid status")
	}

	// Récupérer les informations du signalement
	var report Report
	err := database.DB.QueryRow(`
		SELECT id, type, content_id, status FROM reports
		WHERE id = ?
	`, reportID).Scan(&report.ID, &report.Type, &report.ContentID, &report.Status)
	if err != nil {
		if err == sql.ErrNoRows {
			return errors.New("report not found")
		}
		return err
	}

	// Vérifier que le signalement est en attente
	if report.Status != "pending" {
		return errors.New("report has already been processed")
	}

	// Démarrer une transaction
	tx, err := database.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Mettre à jour le statut du signalement
	_, err = tx.Exec(
		"UPDATE reports SET status = ?, admin_id = ?, admin_response = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		status, adminID, response, reportID,
	)
	if err != nil {
		return err
	}

	// Si le signalement est approuvé, supprimer le contenu signalé
	if status == "approved" {
		var deleteQuery string
		if report.Type == "post" {
			deleteQuery = "DELETE FROM posts WHERE id = ?"
		} else {
			deleteQuery = "DELETE FROM comments WHERE id = ?"
		}
		_, err = tx.Exec(deleteQuery, report.ContentID)
		if err != nil {
			return err
		}
	}

	// Valider la transaction
	return tx.Commit()
}

// UpdateUserRole change le rôle d'un utilisateur (admin, modérateur, utilisateur)
func UpdateUserRole(userID int, newRole string) error {
	// Vérifier que le rôle est valide
	if newRole != "user" && newRole != "moderator" && newRole != "admin" {
		return errors.New("invalid role")
	}

	// Vérifier que l'utilisateur existe
	var count int
	err := database.DB.QueryRow("SELECT COUNT(*) FROM users WHERE id = ?", userID).Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		return errors.New("user not found")
	}

	// Mettre à jour le rôle de l'utilisateur
	_, err = database.DB.Exec("UPDATE users SET role = ? WHERE id = ?", newRole, userID)
	if err != nil {
		return err
	}

	return nil
}

// RequestModeratorRole permet à un utilisateur de demander le rôle de modérateur
func RequestModeratorRole(userID int, reason string) (int, error) {
	// Vérifier que l'utilisateur existe et n'est pas déjà modérateur ou admin
	var role string
	err := database.DB.QueryRow("SELECT role FROM users WHERE id = ?", userID).Scan(&role)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, errors.New("user not found")
		}
		return 0, err
	}
	
	if role == "moderator" || role == "admin" {
		return 0, errors.New("user already has elevated privileges")
	}

	// Créer un rapport spécial pour la demande de modérateur
	result, err := database.DB.Exec(
		"INSERT INTO reports (type, content_id, reporter_id, reason, status) VALUES (?, ?, ?, ?, ?)",
		"moderator_request", userID, userID, reason, "pending",
	)
	if err != nil {
		return 0, err
	}

	requestID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return int(requestID), nil
}