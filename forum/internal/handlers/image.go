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
	"strconv"
	"strings"
)

const (
	MaxImageSize = 20 * 1024 * 1024
	UploadDir    = "./static/uploads"
)

func UploadImageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	currentUser := middleware.GetUserFromContext(r)
	if currentUser == nil {
		http.Error(w, "You must be logged in to upload images", http.StatusUnauthorized)
		return
	}

	if err := os.MkdirAll(UploadDir, 0755); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		log.Printf("Error creating upload directory: %v", err)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, MaxImageSize)
	if err := r.ParseMultipartForm(MaxImageSize); err != nil {
		http.Error(w, "The uploaded file is too big. Please choose a file that's less than 20MB in size", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "Error retrieving the file", http.StatusBadRequest)
		log.Printf("Error retrieving file: %v", err)
		return
	}
	defer file.Close()

	buff := make([]byte, 512)
	_, err = file.Read(buff)
	if err != nil {
		http.Error(w, "Error reading file", http.StatusInternalServerError)
		log.Printf("Error reading file: %v", err)
		return
	}

	file.Seek(0, io.SeekStart)

	filetype := http.DetectContentType(buff)
	if !isAllowedImageType(filetype) {
		http.Error(w, "The provided file format is not allowed. Please upload a JPEG, PNG or GIF image", http.StatusBadRequest)
		return
	}

	filename := fmt.Sprintf("%d_%s", currentUser.ID, header.Filename)
	filename = sanitizeFilename(filename)
	filepath := filepath.Join(UploadDir, filename)

	dst, err := os.Create(filepath)
	if err != nil {
		http.Error(w, "Error saving the file", http.StatusInternalServerError)
		log.Printf("Error creating file: %v", err)
		return
	}
	defer dst.Close()

	if _, err = io.Copy(dst, file); err != nil {
		http.Error(w, "Error saving the file", http.StatusInternalServerError)
		log.Printf("Error copying file: %v", err)
		return
	}

	imageID, err := models.SaveImage(filename, currentUser.ID)
	if err != nil {
		os.Remove(filepath)
		http.Error(w, "Error saving image information", http.StatusInternalServerError)
		log.Printf("Error saving image info: %v", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(fmt.Sprintf(`{"id": %d, "url": "/static/uploads/%s"}`, imageID, filename)))
}

func isAllowedImageType(filetype string) bool {
	allowedTypes := map[string]bool{
		"image/jpeg": true,
		"image/jpg":  true,
		"image/png":  true,
		"image/gif":  true,
	}
	return allowedTypes[filetype]
}

func sanitizeFilename(filename string) string {
	filename = strings.ReplaceAll(filename, " ", "-")
	filename = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '.' || r == '_' {
			return r
		}
		return -1
	}, filename)
	return filename
}

func GetImageHandler(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		http.NotFound(w, r)
		return
	}

	imageID, err := strconv.Atoi(parts[2])
	if err != nil {
		http.NotFound(w, r)
		return
	}

	image, err := models.GetImageByID(strconv.Itoa(imageID))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	filepath := filepath.Join(UploadDir, image.Filename)

	if _, err := os.Stat(filepath); os.IsNotExist(err) {
		http.NotFound(w, r)
		return
	}

	http.ServeFile(w, r, filepath)
}
