package models

import (
	"database/sql"
	"errors"
	"forum/internal/database"
	"time"
)

// Post représente un message publié sur le forum
type Post struct {
	ID        int
	Title     string
	Content   string
	UserID    int
	Username  string // Nom d'utilisateur de l'auteur (non stocké dans la base de données)
	CreatedAt time.Time
	UpdatedAt time.Time
	Categories []Category // Catégories associées au post
	Likes     int         // Nombre de likes (calculé à la demande)
	Dislikes  int         // Nombre de dislikes (calculé à la demande)
	UserReaction string   // Reaction de l'utilisateur actuel: 'like', 'dislike' ou '' (vide)
}

// Category représente une catégorie de posts
type Category struct {
	ID          int
	Name        string
	Description sql.NullString // Modifié pour gérer les valeurs NULL
}

// Comment représente un commentaire sur un post
type Comment struct {
	ID        int
	Content   string
	UserID    int
	Username  string // Nom d'utilisateur de l'auteur (non stocké dans la base de données)
	PostID    int
	CreatedAt time.Time
	UpdatedAt time.Time
	Likes     int    // Nombre de likes (calculé à la demande)
	Dislikes  int    // Nombre de dislikes (calculé à la demande)
	UserReaction string // Reaction de l'utilisateur actuel: 'like', 'dislike' ou '' (vide)
}

// CreatePost crée un nouveau post dans la base de données
func CreatePost(title, content string, userID int, categoryIDs []int) (int, error) {
	// Vérifier si le titre et le contenu ne sont pas vides
	if title == "" || content == "" {
		return 0, errors.New("title and content are required")
	}

	// Démarrer une transaction pour s'assurer que toutes les opérations réussissent ou échouent ensemble
	tx, err := database.DB.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() // En cas d'erreur, annuler toutes les opérations

	// Insérer le post dans la base de données
	result, err := tx.Exec(
		"INSERT INTO posts (title, content, user_id) VALUES (?, ?, ?)",
		title, content, userID,
	)
	if err != nil {
		return 0, err
	}

	// Récupérer l'ID du post nouvellement créé
	postID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	// Associer les catégories au post
	for _, categoryID := range categoryIDs {
		_, err = tx.Exec(
			"INSERT INTO post_categories (post_id, category_id) VALUES (?, ?)",
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

// GetPostByID récupère un post par son ID, avec ses catégories et ses statistiques
func GetPostByID(postID int, currentUserID int) (*Post, error) {
	// Récupérer les informations de base du post
	post := &Post{}
	err := database.DB.QueryRow(`
		SELECT p.id, p.title, p.content, p.user_id, u.username, p.created_at, p.updated_at
		FROM posts p
		JOIN users u ON p.user_id = u.id
		WHERE p.id = ?
	`, postID).Scan(
		&post.ID, &post.Title, &post.Content, &post.UserID, &post.Username, &post.CreatedAt, &post.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("post not found")
		}
		return nil, err
	}

	// Récupérer les catégories associées au post
	rows, err := database.DB.Query(`
		SELECT c.id, c.name, c.description
		FROM categories c
		JOIN post_categories pc ON c.id = pc.category_id
		WHERE pc.post_id = ?
	`, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Stocker les catégories dans le post
	post.Categories = []Category{}
	for rows.Next() {
		var category Category
		err = rows.Scan(&category.ID, &category.Name, &category.Description)
		if err != nil {
			return nil, err
		}
		post.Categories = append(post.Categories, category)
	}

	// Compter les likes et dislikes
	err = database.DB.QueryRow(`
		SELECT 
			(SELECT COUNT(*) FROM post_reactions WHERE post_id = ? AND reaction_type = 'like'),
			(SELECT COUNT(*) FROM post_reactions WHERE post_id = ? AND reaction_type = 'dislike')
	`, postID, postID).Scan(&post.Likes, &post.Dislikes)
	if err != nil {
		return nil, err
	}

	// Vérifier si l'utilisateur actuel a réagi au post
	if currentUserID > 0 {
		var reactionType sql.NullString
		err = database.DB.QueryRow(`
			SELECT reaction_type FROM post_reactions
			WHERE user_id = ? AND post_id = ?
		`, currentUserID, postID).Scan(&reactionType)
		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}
		if reactionType.Valid {
			post.UserReaction = reactionType.String
		}
	}

	return post, nil
}

// GetPosts récupère une liste de posts, avec pagination et filtrage optionnels
func GetPosts(page, perPage int, categoryID int, userID int, sortBy string, currentUserID int) ([]*Post, int, error) {
	// Calculer l'offset pour la pagination
	offset := (page - 1) * perPage

	// Préparer les parties de la requête SQL
	query := `
		SELECT p.id, p.title, p.content, p.user_id, u.username, p.created_at, p.updated_at
		FROM posts p
		JOIN users u ON p.user_id = u.id
	`
	countQuery := `SELECT COUNT(*) FROM posts p`
	
	// Ajouter des conditions si nécessaire
	var conditions []string
	var args []interface{}
	
	if categoryID > 0 {
		query += ` JOIN post_categories pc ON p.id = pc.post_id`
		countQuery += ` JOIN post_categories pc ON p.id = pc.post_id`
		conditions = append(conditions, `pc.category_id = ?`)
		args = append(args, categoryID)
	}
	
	if userID > 0 {
		conditions = append(conditions, `p.user_id = ?`)
		args = append(args, userID)
	}
	
	if len(conditions) > 0 {
		query += ` WHERE ` + conditions[0]
		countQuery += ` WHERE ` + conditions[0]
		for i := 1; i < len(conditions); i++ {
			query += ` AND ` + conditions[i]
			countQuery += ` AND ` + conditions[i]
		}
	}
	
	// Ajouter le tri
	switch sortBy {
	case "likes":
		query += ` ORDER BY (SELECT COUNT(*) FROM post_reactions WHERE post_id = p.id AND reaction_type = 'like') DESC`
	case "date_desc":
		query += ` ORDER BY p.created_at DESC`
	case "date_asc":
		query += ` ORDER BY p.created_at ASC`
	default:
		query += ` ORDER BY p.created_at DESC`
	}
	
	// Ajouter la pagination
	query += ` LIMIT ? OFFSET ?`
	args = append(args, perPage, offset)
	
	// Exécuter la requête pour obtenir le nombre total de posts
	var total int
	err := database.DB.QueryRow(countQuery, args[:len(args)-2]...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}
	
	// Exécuter la requête principale
	rows, err := database.DB.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	
	// Récupérer les résultats
	var posts []*Post
	for rows.Next() {
		post := &Post{}
		err = rows.Scan(
			&post.ID, &post.Title, &post.Content, &post.UserID, &post.Username, &post.CreatedAt, &post.UpdatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		
		// Récupérer les catégories pour ce post
		categoryRows, err := database.DB.Query(`
			SELECT c.id, c.name, c.description
			FROM categories c
			JOIN post_categories pc ON c.id = pc.category_id
			WHERE pc.post_id = ?
		`, post.ID)
		if err != nil {
			return nil, 0, err
		}
		
		post.Categories = []Category{}
		for categoryRows.Next() {
			var category Category
			err = categoryRows.Scan(&category.ID, &category.Name, &category.Description)
			if err != nil {
				categoryRows.Close()
				return nil, 0, err
			}
			post.Categories = append(post.Categories, category)
		}
		categoryRows.Close()
		
		// Compter les likes et dislikes
		err = database.DB.QueryRow(`
			SELECT 
				(SELECT COUNT(*) FROM post_reactions WHERE post_id = ? AND reaction_type = 'like'),
				(SELECT COUNT(*) FROM post_reactions WHERE post_id = ? AND reaction_type = 'dislike')
		`, post.ID, post.ID).Scan(&post.Likes, &post.Dislikes)
		if err != nil {
			return nil, 0, err
		}
		
		// Vérifier si l'utilisateur actuel a réagi au post
		if currentUserID > 0 {
			var reactionType sql.NullString
			err = database.DB.QueryRow(`
				SELECT reaction_type FROM post_reactions
				WHERE user_id = ? AND post_id = ?
			`, currentUserID, post.ID).Scan(&reactionType)
			if err != nil && err != sql.ErrNoRows {
				return nil, 0, err
			}
			if reactionType.Valid {
				post.UserReaction = reactionType.String
			}
		}
		
		posts = append(posts, post)
	}
	
	return posts, total, nil
}

// UpdatePost met à jour un post existant
func UpdatePost(postID, userID int, title, content string, categoryIDs []int) error {
	// Vérifier si le post existe et appartient à l'utilisateur
	var count int
	err := database.DB.QueryRow(
		"SELECT COUNT(*) FROM posts WHERE id = ? AND user_id = ?",
		postID, userID,
	).Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		return errors.New("post not found or you don't have permission to edit it")
	}
	
	// Démarrer une transaction
	tx, err := database.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	
	// Mettre à jour le post
	_, err = tx.Exec(
		"UPDATE posts SET title = ?, content = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		title, content, postID,
	)
	if err != nil {
		return err
	}
	
	// Supprimer les anciennes catégories
	_, err = tx.Exec("DELETE FROM post_categories WHERE post_id = ?", postID)
	if err != nil {
		return err
	}
	
	// Ajouter les nouvelles catégories
	for _, categoryID := range categoryIDs {
		_, err = tx.Exec(
			"INSERT INTO post_categories (post_id, category_id) VALUES (?, ?)",
			postID, categoryID,
		)
		if err != nil {
			return err
		}
	}
	
	// Valider la transaction
	return tx.Commit()
}

// DeletePost supprime un post
func DeletePost(postID, userID int, isAdmin bool) error {
	// Vérifier si le post existe et si l'utilisateur a le droit de le supprimer
	var count int
	var query string
	var args []interface{}
	
	if isAdmin {
		// Un administrateur peut supprimer n'importe quel post
		query = "SELECT COUNT(*) FROM posts WHERE id = ?"
		args = []interface{}{postID}
	} else {
		// Un utilisateur normal ne peut supprimer que ses propres posts
		query = "SELECT COUNT(*) FROM posts WHERE id = ? AND user_id = ?"
		args = []interface{}{postID, userID}
	}
	
	err := database.DB.QueryRow(query, args...).Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		return errors.New("post not found or you don't have permission to delete it")
	}
	
	// Supprimer le post (les contraintes de clé étrangère CASCADE supprimeront aussi les catégories, commentaires et réactions)
	_, err = database.DB.Exec("DELETE FROM posts WHERE id = ?", postID)
	return err
}

// ReactToPost ajoute, modifie ou supprime une réaction (like/dislike) à un post
func ReactToPost(postID, userID int, reactionType string) error {
	// Vérifier que le type de réaction est valide
	if reactionType != "like" && reactionType != "dislike" && reactionType != "" {
		return errors.New("invalid reaction type")
	}
	
	// Vérifier si l'utilisateur a déjà réagi à ce post
	var existingReaction string
	err := database.DB.QueryRow(
		"SELECT reaction_type FROM post_reactions WHERE user_id = ? AND post_id = ?",
		userID, postID,
	).Scan(&existingReaction)
	
	// Si une réaction existe déjà
	if err == nil {
		// Si la nouvelle réaction est vide, supprimer la réaction existante
		if reactionType == "" {
			_, err = database.DB.Exec(
				"DELETE FROM post_reactions WHERE user_id = ? AND post_id = ?",
				userID, postID,
			)
			return err
		}
		
		// Si la réaction est différente, la mettre à jour
		if existingReaction != reactionType {
			_, err = database.DB.Exec(
				"UPDATE post_reactions SET reaction_type = ? WHERE user_id = ? AND post_id = ?",
				reactionType, userID, postID,
			)
			return err
		}
		
		// Si la réaction est la même, ne rien faire
		return nil
	}
	
	// Si aucune réaction n'existe et que la nouvelle est vide, ne rien faire
	if err == sql.ErrNoRows && reactionType == "" {
		return nil
	}
	
	// Sinon, ajouter la nouvelle réaction
	_, err = database.DB.Exec(
		"INSERT INTO post_reactions (user_id, post_id, reaction_type) VALUES (?, ?, ?)",
		userID, postID, reactionType,
	)
	return err
}

// GetAllCategories récupère toutes les catégories disponibles
func GetAllCategories() ([]Category, error) {
	rows, err := database.DB.Query("SELECT id, name, description FROM categories")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var categories []Category
	for rows.Next() {
		var category Category
		err = rows.Scan(&category.ID, &category.Name, &category.Description)
		if err != nil {
			return nil, err
		}
		categories = append(categories, category)
	}
	
	return categories, nil
}

// CreateComment crée un nouveau commentaire sur un post
func CreateComment(content string, userID, postID int) (int, error) {
	// Vérifier si le contenu n'est pas vide
	if content == "" {
		return 0, errors.New("comment content is required")
	}
	
	// Insérer le commentaire dans la base de données
	result, err := database.DB.Exec(
		"INSERT INTO comments (content, user_id, post_id) VALUES (?, ?, ?)",
		content, userID, postID,
	)
	if err != nil {
		return 0, err
	}
	
	// Récupérer l'ID du commentaire nouvellement créé
	commentID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	
	return int(commentID), nil
}

// GetCommentsByPostID récupère tous les commentaires d'un post
func GetCommentsByPostID(postID, currentUserID int) ([]*Comment, error) {
	rows, err := database.DB.Query(`
		SELECT c.id, c.content, c.user_id, u.username, c.post_id, c.created_at, c.updated_at
		FROM comments c
		JOIN users u ON c.user_id = u.id
		WHERE c.post_id = ?
		ORDER BY c.created_at ASC
	`, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var comments []*Comment
	for rows.Next() {
		comment := &Comment{}
		err = rows.Scan(
			&comment.ID, &comment.Content, &comment.UserID, &comment.Username, &comment.PostID, &comment.CreatedAt, &comment.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		
		// Compter les likes et dislikes
		err = database.DB.QueryRow(`
			SELECT 
				(SELECT COUNT(*) FROM comment_reactions WHERE comment_id = ? AND reaction_type = 'like'),
				(SELECT COUNT(*) FROM comment_reactions WHERE comment_id = ? AND reaction_type = 'dislike')
		`, comment.ID, comment.ID).Scan(&comment.Likes, &comment.Dislikes)
		if err != nil {
			return nil, err
		}
		
		// Vérifier si l'utilisateur actuel a réagi au commentaire
		if currentUserID > 0 {
			var reactionType sql.NullString
			err = database.DB.QueryRow(`
				SELECT reaction_type FROM comment_reactions
				WHERE user_id = ? AND comment_id = ?
			`, currentUserID, comment.ID).Scan(&reactionType)
			if err != nil && err != sql.ErrNoRows {
				return nil, err
			}
			if reactionType.Valid {
				comment.UserReaction = reactionType.String
			}
		}
		
		comments = append(comments, comment)
	}
	
	return comments, nil
}

// UpdateComment met à jour un commentaire existant
func UpdateComment(commentID, userID int, content string) error {
	// Vérifier si le commentaire existe et appartient à l'utilisateur
	var count int
	err := database.DB.QueryRow(
		"SELECT COUNT(*) FROM comments WHERE id = ? AND user_id = ?",
		commentID, userID,
	).Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		return errors.New("comment not found or you don't have permission to edit it")
	}
	
	// Mettre à jour le commentaire
	_, err = database.DB.Exec(
		"UPDATE comments SET content = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		content, commentID,
	)
	return err
}

// DeleteComment supprime un commentaire
func DeleteComment(commentID, userID int, isAdmin bool) error {
	// Vérifier si le commentaire existe et si l'utilisateur a le droit de le supprimer
	var count int
	var query string
	var args []interface{}
	
	if isAdmin {
		// Un administrateur peut supprimer n'importe quel commentaire
		query = "SELECT COUNT(*) FROM comments WHERE id = ?"
		args = []interface{}{commentID}
	} else {
		// Un utilisateur normal ne peut supprimer que ses propres commentaires
		query = "SELECT COUNT(*) FROM comments WHERE id = ? AND user_id = ?"
		args = []interface{}{commentID, userID}
	}
	
	err := database.DB.QueryRow(query, args...).Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		return errors.New("comment not found or you don't have permission to delete it")
	}
	
	// Supprimer le commentaire (les contraintes CASCADE supprimeront aussi les réactions)
	_, err = database.DB.Exec("DELETE FROM comments WHERE id = ?", commentID)
	return err
}

// ReactToComment ajoute, modifie ou supprime une réaction (like/dislike) à un commentaire
func ReactToComment(commentID, userID int, reactionType string) error {
	// Vérifier que le type de réaction est valide
	if reactionType != "like" && reactionType != "dislike" && reactionType != "" {
		return errors.New("invalid reaction type")
	}
	
	// Vérifier si l'utilisateur a déjà réagi à ce commentaire
	var existingReaction string
	err := database.DB.QueryRow(
		"SELECT reaction_type FROM comment_reactions WHERE user_id = ? AND comment_id = ?",
		userID, commentID,
	).Scan(&existingReaction)
	
	// Si une réaction existe déjà
	if err == nil {
		// Si la nouvelle réaction est vide, supprimer la réaction existante
		if reactionType == "" {
			_, err = database.DB.Exec(
				"DELETE FROM comment_reactions WHERE user_id = ? AND comment_id = ?",
				userID, commentID,
			)
			return err
		}
		
		// Si la réaction est différente, la mettre à jour
		if existingReaction != reactionType {
			_, err = database.DB.Exec(
				"UPDATE comment_reactions SET reaction_type = ? WHERE user_id = ? AND comment_id = ?",
				reactionType, userID, commentID,
			)
			return err
		}
		
		// Si la réaction est la même, ne rien faire
		return nil
	}
	
	// Si aucune réaction n'existe et que la nouvelle est vide, ne rien faire
	if err == sql.ErrNoRows && reactionType == "" {
		return nil
	}
	
	// Sinon, ajouter la nouvelle réaction
	_, err = database.DB.Exec(
		"INSERT INTO comment_reactions (user_id, comment_id, reaction_type) VALUES (?, ?, ?)",
		userID, commentID, reactionType,
	)
	return err
}