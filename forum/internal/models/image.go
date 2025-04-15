package models

import (
	"database/sql"
	"errors"
	"forum/internal/database"
	"time"
)

// Image représente une image uploadée
type Image struct {
	ID        int
	Filename  string
	UserID    int
	PostID    sql.NullInt64
	CreatedAt time.Time
}

// InitImageTables initialise les tables pour la gestion des images
func InitImageTables() error {
	// Création de la table pour les images
	imagesTable := `
	CREATE TABLE IF NOT EXISTS images (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		filename TEXT NOT NULL,
		user_id INTEGER NOT NULL,
		post_id INTEGER,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (user_id) REFERENCES users(id),
		FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE SET NULL
	);`
	
	_, err := database.DB.Exec(imagesTable)
	if err != nil {
		return err
	}
	
	return nil
}

// GetPostImage récupère l'image associée à un post
func GetPostImage(postID int) (*Image, error) {
	image := &Image{}
	
	err := database.DB.QueryRow(`
		SELECT id, filename, user_id, post_id, created_at
		FROM images
		WHERE post_id = ?
		LIMIT 1
	`, postID).Scan(&image.ID, &image.Filename, &image.UserID, &image.PostID, &image.CreatedAt)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("image not found")
		}
		return nil, err
	}
	
	return image, nil
}

// DeletePostImage supprime l'association entre une image et un post
func DeletePostImage(postID int) error {
	// Récupérer l'image
	image, err := GetPostImage(postID)
	if err != nil {
		return err
	}
	
	// Mettre à jour l'enregistrement pour supprimer l'association
	_, err = database.DB.Exec("UPDATE images SET post_id = NULL WHERE id = ?", image.ID)
	if err != nil {
		return err
	}
	
	return nil
}

// SaveImage enregistre une nouvelle image dans la base de données
func SaveImage(filename string, userID int) (int, error) {
	// Vérifier les entrées
	if filename == "" {
		return 0, errors.New("filename is required")
	}
	
	// Insérer l'image dans la base de données
	result, err := database.DB.Exec(
		"INSERT INTO images (filename, user_id) VALUES (?, ?)",
		filename, userID,
	)
	if err != nil {
		return 0, err
	}
	
	// Récupérer l'ID de l'image
	imageID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	
	return int(imageID), nil
}

// GetImageByID récupère une image par son ID
func GetImageByID(imageID string) (*Image, error) {
	image := &Image{}
	
	err := database.DB.QueryRow(`
		SELECT id, filename, user_id, post_id, created_at
		FROM images
		WHERE id = ?
	`, imageID).Scan(&image.ID, &image.Filename, &image.UserID, &image.PostID, &image.CreatedAt)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("image not found")
		}
		return nil, err
	}
	
	return image, nil
}

// GetImagesByPostID récupère toutes les images associées à un post
func GetImagesByPostID(postID int) ([]*Image, error) {
	rows, err := database.DB.Query(`
		SELECT id, filename, user_id, post_id, created_at
		FROM images
		WHERE post_id = ?
		ORDER BY created_at DESC
	`, postID)
	
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var images []*Image
	for rows.Next() {
		image := &Image{}
		err = rows.Scan(&image.ID, &image.Filename, &image.UserID, &image.PostID, &image.CreatedAt)
		if err != nil {
			return nil, err
		}
		images = append(images, image)
	}
	
	return images, nil
}

// GetImagesByUserID récupère toutes les images téléchargées par un utilisateur
func GetImagesByUserID(userID int) ([]*Image, error) {
	rows, err := database.DB.Query(`
		SELECT id, filename, user_id, post_id, created_at
		FROM images
		WHERE user_id = ?
		ORDER BY created_at DESC
	`, userID)
	
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var images []*Image
	for rows.Next() {
		image := &Image{}
		err = rows.Scan(&image.ID, &image.Filename, &image.UserID, &image.PostID, &image.CreatedAt)
		if err != nil {
			return nil, err
		}
		images = append(images, image)
	}
	
	return images, nil
}

// AssociateImageWithPost associe une image à un post
func AssociateImageWithPost(imageID, postID int) error {
	// Vérifier si l'image existe
	var count int
	err := database.DB.QueryRow("SELECT COUNT(*) FROM images WHERE id = ?", imageID).Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		return errors.New("image not found")
	}
	
	// Vérifier si le post existe
	err = database.DB.QueryRow("SELECT COUNT(*) FROM posts WHERE id = ?", postID).Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		return errors.New("post not found")
	}
	
	// Associer l'image au post
	_, err = database.DB.Exec("UPDATE images SET post_id = ? WHERE id = ?", postID, imageID)
	if err != nil {
		return err
	}
	
	return nil
}

// DeleteImage supprime une image
func DeleteImage(imageID, userID int, isAdmin bool) error {
	// Vérifier si l'image existe et si l'utilisateur a le droit de la supprimer
	var ownerID int
	var filename string
	
	err := database.DB.QueryRow("SELECT user_id, filename FROM images WHERE id = ?", imageID).Scan(&ownerID, &filename)
	if err != nil {
		if err == sql.ErrNoRows {
			return errors.New("image not found")
		}
		return err
	}
	
	// Vérifier les droits de suppression
	if !isAdmin && ownerID != userID {
		return errors.New("you don't have permission to delete this image")
	}
	
	// Supprimer l'image de la base de données
	_, err = database.DB.Exec("DELETE FROM images WHERE id = ?", imageID)
	if err != nil {
		return err
	}
	
	return nil
}

// GetUnassociatedImagesByUserID récupère les images d'un utilisateur qui ne sont pas associées à un post
func GetUnassociatedImagesByUserID(userID int) ([]*Image, error) {
	rows, err := database.DB.Query(`
		SELECT id, filename, user_id, post_id, created_at
		FROM images
		WHERE user_id = ? AND post_id IS NULL
		ORDER BY created_at DESC
	`, userID)
	
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var images []*Image
	for rows.Next() {
		image := &Image{}
		err = rows.Scan(&image.ID, &image.Filename, &image.UserID, &image.PostID, &image.CreatedAt)
		if err != nil {
			return nil, err
		}
		images = append(images, image)
	}
	
	return images, nil
}