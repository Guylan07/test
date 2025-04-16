package models

import (
	"forum/internal/database"
	"time"
)

type UserActivity struct {
	Type      string
	ID        int
	Content   string
	PostID    int
	PostTitle string
	CreatedAt time.Time
	Reaction  string
}

func GetUserPosts(userID int) ([]*Post, error) {
	rows, err := database.DB.Query(`
		SELECT p.id, p.title, p.content, p.user_id, u.username, p.created_at, p.updated_at
		FROM posts p
		JOIN users u ON p.user_id = u.id
		WHERE p.user_id = ?
		ORDER BY p.created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []*Post
	for rows.Next() {
		post := &Post{}
		err = rows.Scan(&post.ID, &post.Title, &post.Content, &post.UserID, &post.Username, &post.CreatedAt, &post.UpdatedAt)
		if err != nil {
			return nil, err
		}

		categoryRows, err := database.DB.Query(`
			SELECT c.id, c.name, c.description
			FROM categories c
			JOIN post_categories pc ON c.id = pc.category_id
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

		err = database.DB.QueryRow(`
			SELECT 
				(SELECT COUNT(*) FROM post_reactions WHERE post_id = ? AND reaction_type = 'like'),
				(SELECT COUNT(*) FROM post_reactions WHERE post_id = ? AND reaction_type = 'dislike')
		`, post.ID, post.ID).Scan(&post.Likes, &post.Dislikes)
		if err != nil {
			return nil, err
		}

		var reactionType string
		err = database.DB.QueryRow(`
			SELECT reaction_type FROM post_reactions
			WHERE user_id = ? AND post_id = ?
		`, userID, post.ID).Scan(&reactionType)
		if err == nil {
			post.UserReaction = reactionType
		}

		posts = append(posts, post)
	}

	return posts, nil
}

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
		return nil, err
	}
	defer rows.Close()

	var comments []*Comment
	for rows.Next() {
		comment := &Comment{}
		var postTitle string
		err = rows.Scan(&comment.ID, &comment.Content, &comment.UserID, &comment.Username, &comment.PostID, &postTitle, &comment.CreatedAt, &comment.UpdatedAt)
		if err != nil {
			return nil, err
		}

		err = database.DB.QueryRow(`
			SELECT 
				(SELECT COUNT(*) FROM comment_reactions WHERE comment_id = ? AND reaction_type = 'like'),
				(SELECT COUNT(*) FROM comment_reactions WHERE comment_id = ? AND reaction_type = 'dislike')
		`, comment.ID, comment.ID).Scan(&comment.Likes, &comment.Dislikes)
		if err != nil {
			return nil, err
		}

		var reactionType string
		err = database.DB.QueryRow(`
			SELECT reaction_type FROM comment_reactions
			WHERE user_id = ? AND comment_id = ?
		`, userID, comment.ID).Scan(&reactionType)
		if err == nil {
			comment.UserReaction = reactionType
		}

		comments = append(comments, comment)
	}

	return comments, nil
}

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
		return nil, err
	}
	defer rows.Close()

	var posts []*Post
	for rows.Next() {
		post := &Post{}
		var reactionType string
		err = rows.Scan(&post.ID, &post.Title, &post.Content, &post.UserID, &post.Username, &post.CreatedAt, &post.UpdatedAt, &reactionType)
		if err != nil {
			return nil, err
		}
		post.UserReaction = reactionType

		categoryRows, err := database.DB.Query(`
			SELECT c.id, c.name, c.description
			FROM categories c
			JOIN post_categories pc ON c.id = pc.category_id
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

		err = database.DB.QueryRow(`
			SELECT 
				(SELECT COUNT(*) FROM post_reactions WHERE post_id = ? AND reaction_type = 'like'),
				(SELECT COUNT(*) FROM post_reactions WHERE post_id = ? AND reaction_type = 'dislike')
		`, post.ID, post.ID).Scan(&post.Likes, &post.Dislikes)
		if err != nil {
			return nil, err
		}

		posts = append(posts, post)
	}

	return posts, nil
}

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
		return nil, err
	}
	defer rows.Close()

	var posts []*Post
	for rows.Next() {
		post := &Post{}
		var reactionType string
		err = rows.Scan(&post.ID, &post.Title, &post.Content, &post.UserID, &post.Username, &post.CreatedAt, &post.UpdatedAt, &reactionType)
		if err != nil {
			return nil, err
		}
		post.UserReaction = reactionType

		categoryRows, err := database.DB.Query(`
			SELECT c.id, c.name, c.description
			FROM categories c
			JOIN post_categories pc ON c.id = pc.category_id
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

		err = database.DB.QueryRow(`
			SELECT 
				(SELECT COUNT(*) FROM post_reactions WHERE post_id = ? AND reaction_type = 'like'),
				(SELECT COUNT(*) FROM post_reactions WHERE post_id = ? AND reaction_type = 'dislike')
		`, post.ID, post.ID).Scan(&post.Likes, &post.Dislikes)
		if err != nil {
			return nil, err
		}

		posts = append(posts, post)
	}

	return posts, nil
}

func GetUserActivity(userID int, limit int) ([]*UserActivity, error) {
	query := `
		SELECT 'post', p.id, p.title, p.id, p.title, p.created_at, NULL
		FROM posts p
		WHERE p.user_id = ?
		
		UNION ALL
		
		SELECT 'comment', c.id, c.content, c.post_id, p.title, c.created_at, NULL
		FROM comments c
		JOIN posts p ON c.post_id = p.id
		WHERE c.user_id = ?
		
		UNION ALL
		
		SELECT 'post_reaction', pr.post_id, NULL, pr.post_id, p.title, pr.created_at, pr.reaction_type
		FROM post_reactions pr
		JOIN posts p ON pr.post_id = p.id
		WHERE pr.user_id = ?
		
		UNION ALL
		
		SELECT 'comment_reaction', cr.comment_id, c.content, c.post_id, p.title, cr.created_at, cr.reaction_type
		FROM comment_reactions cr
		JOIN comments c ON cr.comment_id = c.id
		JOIN posts p ON c.post_id = p.id
		WHERE cr.user_id = ?
		
		ORDER BY created_at DESC
		LIMIT ?
	`

	rows, err := database.DB.Query(query, userID, userID, userID, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var activities []*UserActivity
	for rows.Next() {
		activity := &UserActivity{}
		err = rows.Scan(&activity.Type, &activity.ID, &activity.Content, &activity.PostID, &activity.PostTitle, &activity.CreatedAt, &activity.Reaction)
		if err != nil {
			return nil, err
		}
		activities = append(activities, activity)
	}

	return activities, nil
}
