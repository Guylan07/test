package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// Définition d'une structure pour stocker les informations de limitation de débit
type visitor struct {
	lastSeen time.Time
	requests int
}

// RateLimiter gère la limitation de débit pour les requêtes
type RateLimiter struct {
	visitors    map[string]*visitor
	mu          sync.Mutex
	maxRequests int           // Nombre maximum de requêtes autorisées
	interval    time.Duration // Fenêtre de temps pour le comptage des requêtes
}

// NewRateLimiter crée un nouveau RateLimiter
func NewRateLimiter(maxRequests int, interval time.Duration) *RateLimiter {
	return &RateLimiter{
		visitors:    make(map[string]*visitor),
		maxRequests: maxRequests,
		interval:    interval,
	}
}

// Middleware retourne un middleware HTTP qui limite le débit
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	// Lancer une goroutine pour nettoyer les visiteurs expirés
	go rl.cleanupVisitors()

	// Retourner le middleware
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Obtenir l'adresse IP du client
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}

		// Vérifier si le client a dépassé la limite
		if rl.isRateLimited(ip) {
			// Si la limite est dépassée, renvoyer une erreur 429 Too Many Requests
			w.Header().Set("Retry-After", "60")
			http.Error(w, "Rate limit exceeded. Please try again later.", http.StatusTooManyRequests)
			return
		}

		// Sinon, passer à la prochaine étape du traitement
		next.ServeHTTP(w, r)
	})
}

// isRateLimited vérifie si un client a dépassé la limite de requêtes
func (rl *RateLimiter) isRateLimited(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	// Obtenir le visiteur ou en créer un nouveau
	v, exists := rl.visitors[ip]
	if !exists {
		rl.visitors[ip] = &visitor{
			lastSeen: now,
			requests: 1,
		}
		return false
	}

	// Réinitialiser le compteur si l'intervalle est passé
	if now.Sub(v.lastSeen) > rl.interval {
		v.lastSeen = now
		v.requests = 1
		return false
	}

	// Incrémenter le compteur et vérifier la limite
	v.requests++
	v.lastSeen = now
	
	return v.requests > rl.maxRequests
}

// cleanupVisitors supprime les visiteurs qui n'ont pas été vus depuis un certain temps
func (rl *RateLimiter) cleanupVisitors() {
	for {
		time.Sleep(time.Minute)

		rl.mu.Lock()
		for ip, v := range rl.visitors {
			if time.Since(v.lastSeen) > rl.interval*2 {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}