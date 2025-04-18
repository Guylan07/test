package database

import (
	"database/sql"
	"log"
	"os"
	"golang.org/x/crypto/bcrypt"
	_ "github.com/mattn/go-sqlite3"
)

// Cette variable DB est accessible dans tout le programme pour interagir avec la base de données
var DB *sql.DB

// La fonction InitDB prépare notre base de données pour qu'on puisse l'utiliser
func InitDB(filepath string) error {
	// On vérifie d'abord si le fichier de base de données existe déjà sur l'ordinateur
	_, err := os.Stat(filepath)
	if os.IsNotExist(err) {
		// Si le fichier n'existe pas, on le crée comme on créerait un nouveau cahier vide
		file, err := os.Create(filepath)
		if err != nil {
			// Si on n'arrive pas à créer le fichier, on signale l'erreur
			return err
		}
		file.Close()
	}
	
	// On ouvre la connexion avec notre base de données SQLite
	db, err := sql.Open("sqlite3", filepath)
	if err != nil {
		// Si on n'arrive pas à ouvrir la base de données, on signale l'erreur
		return err
	}
	DB = db
	
	// On crée les tableaux dans notre base de données si ils n'existent pas encore
	err = createTables()
	if err != nil {
		// Si on n'arrive pas à créer les tableaux, on signale l'erreur
		return err
	}
	
	// On note dans le journal que tout s'est bien passé
	log.Println("Database initialized successfully")
	return nil
}

// La fonction createTables crée les structures nécessaires dans notre base de données
func createTables() error {
	// On crée un tableau pour stocker les informations des utilisateurs
	// Chaque ligne du tableau représentera un utilisateur avec ses informations
	usersTable := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		email TEXT UNIQUE NOT NULL,
		username TEXT UNIQUE NOT NULL,
		password TEXT NOT NULL,
		role TEXT DEFAULT 'user',
		oauth_id TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`
	_, err := DB.Exec(usersTable)
	if err != nil {
		// Si on n'arrive pas à créer le tableau des utilisateurs, on signale l'erreur
		return err
	}
	
	// On crée un tableau pour stocker les sessions de connexion
	sessionsTable := `
	CREATE TABLE IF NOT EXISTS sessions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		uuid TEXT UNIQUE NOT NULL,
		user_id INTEGER NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		expires_at TIMESTAMP NOT NULL,
		FOREIGN KEY (user_id) REFERENCES users(id)
	);`
	_, err = DB.Exec(sessionsTable)
	if err != nil {
		// Si on n'arrive pas à créer le tableau des sessions, on signale l'erreur
		return err
	}
	
	// On crée un tableau pour stocker les catégories de posts
	categoriesTable := `
	CREATE TABLE IF NOT EXISTS categories (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT UNIQUE NOT NULL,
		description TEXT
	);`
	_, err = DB.Exec(categoriesTable)
	if err != nil {
		// Si on n'arrive pas à créer le tableau des catégories, on signale l'erreur
		return err
	}

	// On crée un tableau pour stocker les posts
	postsTable := `
	CREATE TABLE IF NOT EXISTS posts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		content TEXT NOT NULL,
		user_id INTEGER NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (user_id) REFERENCES users(id)
	);`
	_, err = DB.Exec(postsTable)
	if err != nil {
		// Si on n'arrive pas à créer le tableau des posts, on signale l'erreur
		return err
	}

	// On crée un tableau pour associer des catégories aux posts
	postCategoriesTable := `
	CREATE TABLE IF NOT EXISTS post_categories (
		post_id INTEGER NOT NULL,
		category_id INTEGER NOT NULL,
		PRIMARY KEY (post_id, category_id),
		FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
		FOREIGN KEY (category_id) REFERENCES categories(id) ON DELETE CASCADE
	);`
	_, err = DB.Exec(postCategoriesTable)
	if err != nil {
		// Si on n'arrive pas à créer le tableau des associations, on signale l'erreur
		return err
	}

	// On crée un tableau pour stocker les commentaires
	commentsTable := `
	CREATE TABLE IF NOT EXISTS comments (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		content TEXT NOT NULL,
		user_id INTEGER NOT NULL,
		post_id INTEGER NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (user_id) REFERENCES users(id),
		FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE
	);`
	_, err = DB.Exec(commentsTable)
	if err != nil {
		// Si on n'arrive pas à créer le tableau des commentaires, on signale l'erreur
		return err
	}

	// On crée un tableau pour stocker les likes/dislikes des posts
	postReactionsTable := `
	CREATE TABLE IF NOT EXISTS post_reactions (
		user_id INTEGER NOT NULL,
		post_id INTEGER NOT NULL,
		reaction_type TEXT NOT NULL,  -- 'like' ou 'dislike'
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (user_id, post_id),
		FOREIGN KEY (user_id) REFERENCES users(id),
		FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE
	);`
	_, err = DB.Exec(postReactionsTable)
	if err != nil {
		// Si on n'arrive pas à créer le tableau des réactions aux posts, on signale l'erreur
		return err
	}

	// On crée un tableau pour stocker les likes/dislikes des commentaires
	commentReactionsTable := `
	CREATE TABLE IF NOT EXISTS comment_reactions (
		user_id INTEGER NOT NULL,
		comment_id INTEGER NOT NULL,
		reaction_type TEXT NOT NULL,  -- 'like' ou 'dislike'
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (user_id, comment_id),
		FOREIGN KEY (user_id) REFERENCES users(id),
		FOREIGN KEY (comment_id) REFERENCES comments(id) ON DELETE CASCADE
	);`
	_, err = DB.Exec(commentReactionsTable)
	if err != nil {
		// Si on n'arrive pas à créer le tableau des réactions aux commentaires, on signale l'erreur
		return err
	}

	// On ajoute quelques catégories par défaut si elles n'existent pas déjà
	defaultCategories := []string{"Général", "Technologie", "Sport", "Musique", "Cinéma", "Jeux vidéo", "Science", "Art", "Politique", "Autre"}
	for _, category := range defaultCategories {
		// Pour chaque catégorie de notre liste, on l'ajoute si elle n'existe pas déjà
		_, err = DB.Exec("INSERT OR IGNORE INTO categories (name) VALUES (?)", category)
		if err != nil {
			// Si on n'arrive pas à ajouter une catégorie, on signale l'erreur
			return err
		}
	}
	
	// Vérifier si un administrateur existe déjà
	var count int
	err = DB.QueryRow("SELECT COUNT(*) FROM users WHERE role = 'admin'").Scan(&count)
	if err != nil {
		return err
	}
	
	// Si aucun administrateur n'existe, en créer un par défaut
	if count == 0 {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte("Admin123"), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		
		_, err = DB.Exec(`
			INSERT INTO users (email, username, password, role, created_at) 
			VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)`,
			"admin@gmail.com", "Admin", string(hashedPassword), "admin")
		if err != nil {
			return err
		}
		
		log.Println("Default administrator account created successfully")
	}
	
	// Tout s'est bien passé, on ne signale aucune erreur
	return nil
}

// La fonction CloseDB ferme proprement la connexion avec la base de données
func CloseDB() error {
	if DB != nil {
		// Si la base de données est ouverte, on la ferme
		return DB.Close()
	}
	// Si la base de données n'est pas ouverte, il n'y a rien à faire
	return nil
}