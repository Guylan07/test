package handlers

import (
	"fmt"
	"forum/internal/middleware"
	"forum/internal/models"
	"forum/internal/utils"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func HomeHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	currentUser := middleware.GetUserFromContext(r)
	var currentUserID int
	if currentUser != nil {
		currentUserID = currentUser.ID
	}

	page := 1
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if pageNum, err := strconv.Atoi(pageStr); err == nil && pageNum > 0 {
			page = pageNum
		}
	}

	perPage := 10

	categoryID := 0
	if categoryStr := r.URL.Query().Get("category"); categoryStr != "" {
		if catID, err := strconv.Atoi(categoryStr); err == nil && catID > 0 {
			categoryID = catID
		}
	}

	userID := 0
	if userStr := r.URL.Query().Get("user"); userStr != "" {
		if uID, err := strconv.Atoi(userStr); err == nil && uID > 0 {
			userID = uID
		}
	}

	sortBy := r.URL.Query().Get("sort")
	if sortBy == "" {
		sortBy = "date_desc"
	}

	posts, total, err := models.GetPosts(page, perPage, categoryID, userID, sortBy, currentUserID)
	if err != nil {
		http.Error(w, "Error fetching posts", http.StatusInternalServerError)
		log.Printf("Error fetching posts: %v", err)
		return
	}

	categories, err := models.GetAllCategories()
	if err != nil {
		http.Error(w, "Error fetching categories", http.StatusInternalServerError)
		log.Printf("Error fetching categories: %v", err)
		return
	}

	totalPages := (total + perPage - 1) / perPage

	data := map[string]interface{}{
		"Posts":       posts,
		"Categories":  categories,
		"CurrentUser": currentUser,
		"CurrentPage": page,
		"TotalPages":  totalPages,
		"CategoryID":  categoryID,
		"UserID":      userID,
		"SortBy":      sortBy,
	}

	tmpl, err := utils.ParseTemplate("templates/base.html", "templates/home.html")
	if err != nil {
		http.Error(w, "Error loading templates", http.StatusInternalServerError)
		log.Printf("Error parsing template: %v", err)
		return
	}

	err = tmpl.ExecuteTemplate(w, "base", data)
	if err != nil {
		http.Error(w, "Error rendering page", http.StatusInternalServerError)
		log.Printf("Error executing template: %v", err)
	}
}

func CreatePostHandler(w http.ResponseWriter, r *http.Request) {
	currentUser := middleware.GetUserFromContext(r)
	if currentUser == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if r.Method == http.MethodGet {
		categories, err := models.GetAllCategories()
		if err != nil {
			http.Error(w, "Error fetching categories", http.StatusInternalServerError)
			log.Printf("Error fetching categories: %v", err)
			return
		}

		data := map[string]interface{}{
			"Categories":  categories,
			"CurrentUser": currentUser,
		}

		tmpl, err := utils.ParseTemplate("templates/base.html", "templates/create_post.html")
		if err != nil {
			http.Error(w, "Error loading templates", http.StatusInternalServerError)
			log.Printf("Error parsing template: %v", err)
			return
		}

		err = tmpl.ExecuteTemplate(w, "base", data)
		if err != nil {
			http.Error(w, "Error rendering page", http.StatusInternalServerError)
			log.Printf("Error executing template: %v", err)
		}
		return
	}

	if r.Method == http.MethodPost {
		r.Body = http.MaxBytesReader(w, r.Body, 25*1024*1024)
		err := r.ParseMultipartForm(25 * 1024 * 1024)
		if err != nil {
			http.Error(w, "The form is too large. Please reduce the size of your image.", http.StatusBadRequest)
			return
		}

		title := r.FormValue("title")
		content := r.FormValue("content")
		categoryIDs := r.Form["categories"]

		var categoryIDsInt []int
		for _, idStr := range categoryIDs {
			id, err := strconv.Atoi(idStr)
			if err == nil && id > 0 {
				categoryIDsInt = append(categoryIDsInt, id)
			}
		}

		if title == "" || content == "" {
			http.Error(w, "Title and content are required", http.StatusBadRequest)
			return
		}

		postID, err := models.CreatePost(title, content, currentUser.ID, categoryIDsInt)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Printf("Error creating post: %v", err)
			return
		}

		file, header, err := r.FormFile("image")
		if err == nil {
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
				http.Error(w, "Error saving image information", http.StatusInternalServerError)
				log.Printf("Error saving image info: %v", err)
				return
			}

			err = models.AssociateImageWithPost(imageID, postID)
			if err != nil {
				http.Error(w, "Error associating image with post", http.StatusInternalServerError)
				log.Printf("Error associating image: %v", err)
				return
			}
		}

		http.Redirect(w, r, "/post/"+strconv.Itoa(postID), http.StatusSeeOther)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func ViewPostHandler(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) != 3 {
		http.NotFound(w, r)
		return
	}

	postID, err := strconv.Atoi(parts[2])
	if err != nil || postID <= 0 {
		http.NotFound(w, r)
		return
	}

	currentUser := middleware.GetUserFromContext(r)
	var currentUserID int
	if currentUser != nil {
		currentUserID = currentUser.ID
	}

	post, err := models.GetPostByID(postID, currentUserID)
	if err != nil {
		if err.Error() == "post not found" {
			http.NotFound(w, r)
		} else {
			http.Error(w, "Error fetching post", http.StatusInternalServerError)
			log.Printf("Error fetching post: %v", err)
		}
		return
	}

	comments, err := models.GetCommentsByPostID(postID, currentUserID)
	if err != nil {
		http.Error(w, "Error fetching comments", http.StatusInternalServerError)
		log.Printf("Error fetching comments: %v", err)
		return
	}

	postImage, err := models.GetPostImage(postID)
	if err != nil && err.Error() != "image not found" {
		log.Printf("Error fetching post image: %v", err)
	}

	data := map[string]interface{}{
		"Post":        post,
		"PostImage":   postImage,
		"Comments":    comments,
		"CurrentUser": currentUser,
		"CanEdit":     currentUser != nil && (currentUser.ID == post.UserID || currentUser.Role == "admin"),
	}

	tmpl, err := utils.ParseTemplate("templates/base.html", "templates/view_post.html")
	if err != nil {
		http.Error(w, "Error loading templates", http.StatusInternalServerError)
		log.Printf("Error parsing template: %v", err)
		return
	}

	err = tmpl.ExecuteTemplate(w, "base", data)
	if err != nil {
		http.Error(w, "Error rendering page", http.StatusInternalServerError)
		log.Printf("Error executing template: %v", err)
	}
}

func EditPostHandler(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) != 4 || parts[2] != "edit" {
		http.NotFound(w, r)
		return
	}

	postID, err := strconv.Atoi(parts[3])
	if err != nil || postID <= 0 {
		http.NotFound(w, r)
		return
	}

	currentUser := middleware.GetUserFromContext(r)
	if currentUser == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	post, err := models.GetPostByID(postID, currentUser.ID)
	if err != nil {
		if err.Error() == "post not found" {
			http.NotFound(w, r)
		} else {
			http.Error(w, "Error fetching post", http.StatusInternalServerError)
			log.Printf("Error fetching post: %v", err)
		}
		return
	}

	if currentUser.ID != post.UserID && currentUser.Role != "admin" {
		http.Error(w, "You don't have permission to edit this post", http.StatusForbidden)
		return
	}

	postImage, err := models.GetPostImage(postID)
	if err != nil && err.Error() != "image not found" {
		log.Printf("Error fetching post image: %v", err)
	}

	if r.Method == http.MethodGet {
		categories, err := models.GetAllCategories()
		if err != nil {
			http.Error(w, "Error fetching categories", http.StatusInternalServerError)
			log.Printf("Error fetching categories: %v", err)
			return
		}

		var selectedCategoryIDs []int
		for _, cat := range post.Categories {
			selectedCategoryIDs = append(selectedCategoryIDs, cat.ID)
		}

		data := map[string]interface{}{
			"Post":               post,
			"PostImage":          postImage,
			"Categories":         categories,
			"SelectedCategories": selectedCategoryIDs,
			"CurrentUser":        currentUser,
		}

		tmpl, err := utils.ParseTemplate("templates/base.html", "templates/edit_post.html")
		if err != nil {
			http.Error(w, "Error loading templates", http.StatusInternalServerError)
			log.Printf("Error parsing template: %v", err)
			return
		}

		err = tmpl.ExecuteTemplate(w, "base", data)
		if err != nil {
			http.Error(w, "Error rendering page", http.StatusInternalServerError)
			log.Printf("Error executing template: %v", err)
		}
		return
	}

	if r.Method == http.MethodPost {
		r.Body = http.MaxBytesReader(w, r.Body, 25*1024*1024)
		err := r.ParseMultipartForm(25 * 1024 * 1024)
		if err != nil {
			http.Error(w, "The form is too large. Please reduce the size of your image.", http.StatusBadRequest)
			return
		}

		title := r.FormValue("title")
		content := r.FormValue("content")
		categoryIDs := r.Form["categories"]
		removeImage := r.FormValue("remove_image") == "true"

		var categoryIDsInt []int
		for _, idStr := range categoryIDs {
			id, err := strconv.Atoi(idStr)
			if err == nil && id > 0 {
				categoryIDsInt = append(categoryIDsInt, id)
			}
		}

		if title == "" || content == "" {
			http.Error(w, "Title and content are required", http.StatusBadRequest)
			return
		}

		err = models.UpdatePost(postID, currentUser.ID, title, content, categoryIDsInt)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Printf("Error updating post: %v", err)
			return
		}

		if removeImage && postImage != nil {
			err = models.DeletePostImage(postID)
			if err != nil {
				log.Printf("Error removing post image: %v", err)
			}
		} else {
			file, header, err := r.FormFile("image")
			if err == nil {
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

				if postImage != nil {
					err = models.DeletePostImage(postID)
					if err != nil {
						log.Printf("Error removing old post image: %v", err)
					}
				}

				imageID, err := models.SaveImage(filename, currentUser.ID)
				if err != nil {
					http.Error(w, "Error saving image information", http.StatusInternalServerError)
					log.Printf("Error saving image info: %v", err)
					return
				}

				err = models.AssociateImageWithPost(imageID, postID)
				if err != nil {
					http.Error(w, "Error associating image with post", http.StatusInternalServerError)
					log.Printf("Error associating image: %v", err)
					return
				}
			}
		}

		http.Redirect(w, r, "/post/"+strconv.Itoa(postID), http.StatusSeeOther)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func DeletePostHandler(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) != 4 || parts[2] != "delete" {
		http.NotFound(w, r)
		return
	}

	postID, err := strconv.Atoi(parts[3])
	if err != nil || postID <= 0 {
		http.NotFound(w, r)
		return
	}

	currentUser := middleware.GetUserFromContext(r)
	if currentUser == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	isAdmin := currentUser.Role == "admin"
	err = models.DeletePost(postID, currentUser.ID, isAdmin)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Printf("Error deleting post: %v", err)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func ReactToPostHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	currentUser := middleware.GetUserFromContext(r)
	if currentUser == nil {
		http.Error(w, "You must be logged in to react to posts", http.StatusUnauthorized)
		return
	}

	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	postIDStr := r.FormValue("post_id")
	reactionType := r.FormValue("reaction_type")

	postID, err := strconv.Atoi(postIDStr)
	if err != nil || postID <= 0 {
		http.Error(w, "Invalid post ID", http.StatusBadRequest)
		return
	}

	err = models.ReactToPost(postID, currentUser.ID, reactionType)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Printf("Error reacting to post: %v", err)
		return
	}

	http.Redirect(w, r, "/post/"+postIDStr, http.StatusSeeOther)
}