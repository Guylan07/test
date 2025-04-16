package models

import (
	"database/sql"
	"forum/internal/database"
	"log"
	"time"
)

// UserActivity représente une activité d'un utilisateur dans le forum
type UserActivity struct {
	Type      string    // Type d'activité: "post", "comment", "post_reaction", "comment_reaction"
	ID        int       // ID de l'élément concerné (post, commentaire)
	Content   string    // Contenu ou titre (peut être vide)
	PostID    int       // ID du post associé
	PostTitle string    // Titre du post associé
	CreatedAt time.Time // Date de création de l'activité
	Reaction  string    // Type de réaction (like/dislike) si applicable
}

// GetUserPosts récupère tous les posts créés par un utilisateur spécifique
func GetUserPosts(userID int) ([]*Post, error) {
	rows, err := database.DB.Query(`
		SELECT p.id, p.title, p.content, p.user_id, u.username, p.created_at, p.updated_at
		FROM posts p
		JOIN users u ON p.user_id = u.id
		WHERE p.user_id = ?
		ORDER BY p.created_at DESC
	`, userID)
	if err != nil {
		log.Printf("Error querying user posts: %v", err)
		return nil, err
	}
	defer rows.Close()

	var posts []*Post
	for rows.Next() {
		post := &Post{}
		err = rows.Scan(&post.ID, &post.Title, &post.Content, &post.UserID, &post.Username, &post.CreatedAt, &post.UpdatedAt)
		if err != nil {
			log.Printf("Error scanning post row: %v", err)
			return nil, err
		}

		// Récupérer les catégories pour ce post
		categoryRows, err := database.DB.Query(`
			SELECT c.id, c.name, c.description
			FROM categories c
			JOIN post_categories pc ON c.id = pc.category_id
			WHERE pc.post_id = ?
		`, post.ID)
		if err != nil {
			log.Printf("Error querying post categories: %v", err)
			return nil, err
		}

		post.Categories = []Category{}
		for categoryRows.Next() {
			var category Category
			err = categoryRows.Scan(&category.ID, &category.Name, &category.Description)
			if err != nil {
				categoryRows.Close()
				log.Printf("Error scanning category row: %v", err)
				return nil, err
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
			log.Printf("Error counting post reactions: %v", err)
			return nil, err
		}

		// Vérifier si l'utilisateur a réagi au post
		var reactionType string
		err = database.DB.QueryRow(`
			SELECT reaction_type FROM post_reactions
			WHERE user_id = ? AND post_id = ?
		`, userID, post.ID).Scan(&reactionType)
		if err == nil {
			post.UserReaction = reactionType
		} else if err != sql.ErrNoRows {
			log.Printf("Error checking user reaction: %v", err)
			return nil, err
		}

		posts = append(posts, post)
	}

	if err = rows.Err(); err != nil {
		log.Printf("Error iterating through post rows: %v", err)
		return nil, err
	}

	return posts, nil
}

// GetUserComments récupère tous les commentaires créés par un utilisateur spécifique
func GetUserComments(userID int) ([]*Comment, error) {
	rows, err := database.DB.Query(`
		SELECT c.id, c.content, c.user_id, u.username, c.post_id, p.title, c.created_at, c.updated_at
		FROM comments c
		JOIN users u ON c.user_id = u.id
		JOIN posts p ON c.post_id = p.id
		WHERE c.user_id = ?
		ORDER BY c.created_at DESC
	`, userID)
	if err != nil {
		log.Printf("Error querying user comments: %v", err)
		return nil, err
	}
	defer rows.Close()

	var comments []*Comment
	for rows.Next() {
		comment := &Comment{}
		var postTitle string
		err = rows.Scan(&comment.ID, &comment.Content, &comment.UserID, &comment.Username, &comment.PostID, &postTitle, &comment.CreatedAt, &comment.UpdatedAt)
		if err != nil {
			log.Printf("Error scanning comment row: %v", err)
			return nil, err
		}

		// Compter les likes et dislikes
		err = database.DB.QueryRow(`
			SELECT 
				(SELECT COUNT(*) FROM comment_reactions WHERE comment_id = ? AND reaction_type = 'like'),
				(SELECT COUNT(*) FROM comment_reactions WHERE comment_id = ? AND reaction_type = 'dislike')
		`, comment.ID, comment.ID).Scan(&comment.Likes, &comment.Dislikes)
		if err != nil {
			log.Printf("Error counting comment reactions: %v", err)
			return nil, err
		}

		// Vérifier si l'utilisateur a réagi au commentaire
		var reactionType string
		err = database.DB.QueryRow(`
			SELECT reaction_type FROM comment_reactions
			WHERE user_id = ? AND comment_id = ?
		`, userID, comment.ID).Scan(&reactionType)
		if err == nil {
			comment.UserReaction = reactionType
		} else if err != sql.ErrNoRows {
			log.Printf("Error checking user reaction to comment: %v", err)
			return nil, err
		}

		comments = append(comments, comment)
	}

	if err = rows.Err(); err != nil {
		log.Printf("Error iterating through comment rows: %v", err)
		return nil, err
	}

	return comments, nil
}

// GetUserLikedPosts récupère tous les posts aimés par un utilisateur spécifique
func GetUserLikedPosts(userID int) ([]*Post, error) {
	rows, err := database.DB.Query(`
		SELECT p.id, p.title, p.content, p.user_id, u.username, p.created_at, p.updated_at, pr.reaction_type
		FROM posts p
		JOIN users u ON p.user_id = u.id
		JOIN post_reactions pr ON p.id = pr.post_id
		WHERE pr.user_id = ? AND pr.reaction_type = 'like'
		ORDER BY pr.created_at DESC
	`, userID)
	if err != nil {
		log.Printf("Error querying user liked posts: %v", err)
		return nil, err
	}
	defer rows.Close()

	var posts []*Post
	for rows.Next() {
		post := &Post{}
		var reactionType string
		err = rows.Scan(&post.ID, &post.Title, &post.Content, &post.UserID, &post.Username, &post.CreatedAt, &post.UpdatedAt, &reactionType)
		if err != nil {
			log.Printf("Error scanning liked post row: %v", err)
			return nil, err
		}
		post.UserReaction = reactionType

		// Récupérer les catégories pour ce post
		categoryRows, err := database.DB.Query(`
			SELECT c.id, c.name, c.description
			FROM categories c
			JOIN post_categories pc ON c.id = pc.category_id
			WHERE pc.post_id = ?
		`, post.ID)
		if err != nil {
			log.Printf("Error querying liked post categories: %v", err)
			return nil, err
		}

		post.Categories = []Category{}
		for categoryRows.Next() {
			var category Category
			err = categoryRows.Scan(&category.ID, &category.Name, &category.Description)
			if err != nil {
				categoryRows.Close()
				log.Printf("Error scanning category row for liked post: %v", err)
				return nil, err
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
			log.Printf("Error counting reactions for liked post: %v", err)
			return nil, err
		}

		posts = append(posts, post)
	}

	if err = rows.Err(); err != nil {
		log.Printf("Error iterating through liked post rows: %v", err)
		return nil, err
	}

	return posts, nil
}

// GetUserDislikedPosts récupère tous les posts non aimés par un utilisateur spécifique
func GetUserDislikedPosts(userID int) ([]*Post, error) {
	rows, err := database.DB.Query(`
		SELECT p.id, p.title, p.content, p.user_id, u.username, p.created_at, p.updated_at, pr.reaction_type
		FROM posts p
		JOIN users u ON p.user_id = u.id
		JOIN post_reactions pr ON p.id = pr.post_id
		WHERE pr.user_id = ? AND pr.reaction_type = 'dislike'
		ORDER BY pr.created_at DESC
	`, userID)
	if err != nil {
		log.Printf("Error querying user disliked posts: %v", err)
		return nil, err
	}
	defer rows.Close()

	var posts []*Post
	for rows.Next() {
		post := &Post{}
		var reactionType string
		err = rows.Scan(&post.ID, &post.Title, &post.Content, &post.UserID, &post.Username, &post.CreatedAt, &post.UpdatedAt, &reactionType)
		if err != nil {
			log.Printf("Error scanning disliked post row: %v", err)
			return nil, err
		}
		post.UserReaction = reactionType

		// Récupérer les catégories pour ce post
		categoryRows, err := database.DB.Query(`
			SELECT c.id, c.name, c.description
			FROM categories c
			JOIN post_categories pc ON c.id = pc.category_id
			WHERE pc.post_id = ?
		`, post.ID)
		if err != nil {
			log.Printf("Error querying disliked post categories: %v", err)
			return nil, err
		}

		post.Categories = []Category{}
		for categoryRows.Next() {
			var category Category
			err = categoryRows.Scan(&category.ID, &category.Name, &category.Description)
			if err != nil {
				categoryRows.Close()
				log.Printf("Error scanning category row for disliked post: %v", err)
				return nil, err
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
			log.Printf("Error counting reactions for disliked post: %v", err)
			return nil, err
		}

		posts = append(posts, post)
	}

	if err = rows.Err(); err != nil {
		log.Printf("Error iterating through disliked post rows: %v", err)
		return nil, err
	}

	return posts, nil
}

// GetUserActivity récupère un flux d'activités récentes d'un utilisateur
func GetUserActivity(userID int, limit int) ([]*UserActivity, error) {
	// Requête pour obtenir les activités d'un utilisateur avec des alias explicites
	// et en utilisant des jointures appropriées
	query := `
		SELECT 'post' as activity_type, p.id as item_id, p.title as content, p.id as post_id, p.title as post_title, p.created_at as activity_date, NULL as reaction_type
		FROM posts p
		WHERE p.user_id = ?
		
		UNION ALL
		
		SELECT 'comment' as activity_type, c.id as item_id, c.content as content, c.post_id as post_id, p.title as post_title, c.created_at as activity_date, NULL as reaction_type
		FROM comments c
		JOIN posts p ON c.post_id = p.id
		WHERE c.user_id = ?
		
		UNION ALL
		
		SELECT 'post_reaction' as activity_type, pr.post_id as item_id, NULL as content, pr.post_id as post_id, p.title as post_title, pr.created_at as activity_date, pr.reaction_type as reaction_type
		FROM post_reactions pr
		JOIN posts p ON pr.post_id = p.id
		WHERE pr.user_id = ?
		
		UNION ALL
		
		SELECT 'comment_reaction' as activity_type, cr.comment_id as item_id, c.content as content, c.post_id as post_id, p.title as post_title, cr.created_at as activity_date, cr.reaction_type as reaction_type
		FROM comment_reactions cr
		JOIN comments c ON cr.comment_id = c.id
		JOIN posts p ON c.post_id = p.id
		WHERE cr.user_id = ?
		
		ORDER BY activity_date DESC
		LIMIT ?
	`

	// Exécution de la requête avec tous les paramètres
	rows, err := database.DB.Query(query, userID, userID, userID, userID, limit)
	if err != nil {
		log.Printf("Error executing activity query: %v", err)
		return nil, err
	}
	defer rows.Close()

	var activities []*UserActivity
	for rows.Next() {
		activity := &UserActivity{}
		
		// Utilisation de NullString pour les colonnes qui peuvent être NULL
		var content sql.NullString
		var reaction sql.NullString
		
		// Scan des valeurs en gérant correctement les NULL
		err = rows.Scan(
			&activity.Type,
			&activity.ID,
			&content,
			&activity.PostID,
			&activity.PostTitle,
			&activity.CreatedAt,
			&reaction,
		)
		
		if err != nil {
			log.Printf("Error scanning activity row: %v", err)
			return nil, err
		}
		
		// Conversion des NullString en string
		if content.Valid {
			activity.Content = content.String
		}
		
		if reaction.Valid {
			activity.Reaction = reaction.String
		}
		
		activities = append(activities, activity)
	}

	// Vérification des erreurs pendant l'itération
	if err = rows.Err(); err != nil {
		log.Printf("Error iterating activity rows: %v", err)
		return nil, err
	}

	return activities, nil
}