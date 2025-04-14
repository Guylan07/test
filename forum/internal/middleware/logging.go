package middleware

import (
	"log"
	"net/http"
	"time"
)

// LoggingMiddleware enregistre les informations sur chaque requête HTTP
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Enregistrer l'heure de début
		start := time.Now()
		
		// Créer un ResponseWriter personnalisé pour capturer le code de statut HTTP
		lrw := newLoggingResponseWriter(w)
		
		// Traiter la requête
		next.ServeHTTP(lrw, r)
		
		// Calculer la durée
		duration := time.Since(start)
		
		// Enregistrer la requête dans les logs
		log.Printf(
			"%s - %s %s %s - %d - %v",
			r.RemoteAddr,
			r.Method,
			r.URL.Path,
			r.Proto,
			lrw.statusCode,
			duration,
		)
	})
}

// loggingResponseWriter est un wrapper autour de http.ResponseWriter pour capturer le code de statut
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

// newLoggingResponseWriter crée un nouveau loggingResponseWriter
func newLoggingResponseWriter(w http.ResponseWriter) *loggingResponseWriter {
	return &loggingResponseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK, // 200 par défaut
	}
}

// WriteHeader capture le code de statut
func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}