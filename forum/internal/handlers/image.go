package handlers

import (
	"fmt"
	"forum/internal/middleware"
	"forum/internal/models"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	MaxImageSize = 20 * 1024 * 1024 // 20 MB en octets
	UploadDir    = "./static/uploads"
)

// UploadImageHandler gère le téléchargement d'images
func UploadImageHandler(w http.ResponseWriter, r *http.Request) {
	// Vérifier que la méthode est POST
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Vérifier que l'utilisateur est connecté
	currentUser := middleware.GetUserFromContext(r)
	if currentUser == nil {
		http.Error(w, "You must be logged in to upload images", http.StatusUnauthorized)
		return
	}

	// S'assurer que le dossier d'upload existe
	if err := os.MkdirAll(UploadDir, 0755); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		log.Printf("Error creating upload directory: %v", err)
		return
	}

	// Limiter la taille maximale du fichier à 20 MB
	r.Body = http.MaxBytesReader(w, r.Body, MaxImageSize)
	if err := r.ParseMultipartForm(MaxImageSize); err != nil {
		http.Error(w, "The uploaded file is too big. Please choose a file that's less than 20MB in size", http.StatusBadRequest)
		return
	}

	// Récupérer le fichier depuis le formulaire
	file, header, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "Error retrieving the file", http.StatusBadRequest)
		log.Printf("Error retrieving file: %v", err)
		return
	}
	defer file.Close()

	// Valider le type de fichier
	buff := make([]byte, 512)
	_, err = file.Read(buff)
	if err != nil {
		http.Error(w, "Error reading file", http.StatusInternalServerError)
		log.Printf("Error reading file: %v", err)
		return
	}

	// Revenir au début du fichier après avoir lu le header
	file.Seek(0, io.SeekStart)

	// Vérifier le type MIME
	filetype := http.DetectContentType(buff)
	if !isAllowedImageType(filetype) {
		http.Error(w, "The provided file format is not allowed. Please upload a JPEG, PNG or GIF image", http.StatusBadRequest)
		return
	}

	// Créer un nom de fichier unique
	filename := fmt.Sprintf("%d_%s", currentUser.ID, header.Filename)
	// Nettoyer le nom de fichier
	filename = sanitizeFilename(filename)
	// Créer le chemin complet
	filepath := filepath.Join(UploadDir, filename)

	// Créer le fichier sur le serveur
	dst, err := os.Create(filepath)
	if err != nil {
		http.Error(w, "Error saving the file", http.StatusInternalServerError)
		log.Printf("Error creating file: %v", err)
		return
	}
	defer dst.Close()

	// Copier le contenu dans le fichier créé
	if _, err = io.Copy(dst, file); err != nil {
		http.Error(w, "Error saving the file", http.StatusInternalServerError)
		log.Printf("Error copying file: %v", err)
		return
	}

	// Enregistrer l'image dans la base de données
	imageID, err := models.SaveImage(filename, currentUser.ID)
	if err != nil {
		// Supprimer le fichier si l'enregistrement échoue
		os.Remove(filepath)
		http.Error(w, "Error saving image information", http.StatusInternalServerError)
		log.Printf("Error saving image info: %v", err)
		return
	}

	// Renvoyer l'ID de l'image en réponse
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(fmt.Sprintf(`{"id": %d, "url": "/static/uploads/%s"}`, imageID, filename)))
}

// isAllowedImageType vérifie si le type MIME est autorisé
func isAllowedImageType(filetype string) bool {
	allowedTypes := map[string]bool{
		"image/jpeg": true,
		"image/jpg":  true,
		"image/png":  true,
		"image/gif":  true,
	}
	return allowedTypes[filetype]
}

// sanitizeFilename nettoie le nom de fichier
func sanitizeFilename(filename string) string {
	// Remplacer les caractères non alphanumériques par des tirets
	filename = strings.ReplaceAll(filename, " ", "-")
	filename = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '.' || r == '_' {
			return r
		}
		return -1
	}, filename)
	return filename
}

// GetImageHandler récupère une image par son ID
func GetImageHandler(w http.ResponseWriter, r *http.Request) {
	// Extraire l'ID de l'image de l'URL
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		http.NotFound(w, r)
		return
	}

	imageID := parts[2]
	
	// Récupérer les informations de l'image depuis la base de données
	image, err := models.GetImageByID(imageID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Construire le chemin complet vers le fichier
	filepath := filepath.Join(UploadDir, image.Filename)

	// Vérifier que le fichier existe
	if _, err := os.Stat(filepath); os.IsNotExist(err) {
		http.NotFound(w, r)
		return
	}

	// Servir le fichier
	http.ServeFile(w, r, filepath)
}