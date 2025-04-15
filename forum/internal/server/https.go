package server

import (
	"crypto/tls"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"golang.org/x/crypto/acme/autocert"
	"os/exec"
)

// Configuration pour le serveur HTTPS
type HTTPSConfig struct {
	Domain      string // Domaine du site (ex: "monsite.com")
	CertPath    string // Chemin vers le dossier des certificats
	Development bool   // Mode développement (certificat auto-signé)
}

// ConfigureHTTPS configure un serveur HTTPS
func ConfigureHTTPS(handler http.Handler, config HTTPSConfig) *http.Server {
	var server *http.Server

	// Configurer le serveur en fonction du mode (développement ou production)
	if config.Development {
		// En développement, utiliser un certificat auto-signé
		server = &http.Server{
			Addr:    ":8443",
			Handler: handler,
			TLSConfig: &tls.Config{
				MinVersion: tls.VersionTLS12, // Exiger TLS 1.2 minimum
				CipherSuites: []uint16{
					tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
					tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
					tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
					tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
					tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
					tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				},
			},
		}
		log.Println("Configured HTTPS server with self-signed certificate on port 8443")
	} else {
		// En production, utiliser Let's Encrypt pour des certificats valides
		certManager := autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			HostPolicy: autocert.HostWhitelist(config.Domain),
			Cache:      autocert.DirCache(config.CertPath),
		}

		server = &http.Server{
			Addr:    ":443",
			Handler: handler,
			TLSConfig: &tls.Config{
				GetCertificate: certManager.GetCertificate,
				MinVersion:     tls.VersionTLS12,
				CipherSuites: []uint16{
					tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
					tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
					tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
					tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
					tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
					tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				},
			},
		}

		// Configurer également un serveur HTTP qui redirige vers HTTPS
		go func() {
			httpServer := &http.Server{
				Addr:    ":80",
				Handler: certManager.HTTPHandler(http.HandlerFunc(redirectToHTTPS)),
			}
			log.Fatal(httpServer.ListenAndServe())
		}()

		log.Println("Configured HTTPS server with Let's Encrypt certificate on port 443")
	}

	return server
}

// redirectToHTTPS redirige toutes les requêtes HTTP vers HTTPS
func redirectToHTTPS(w http.ResponseWriter, r *http.Request) {
	target := "https://" + r.Host + r.URL.Path
	if len(r.URL.RawQuery) > 0 {
		target += "?" + r.URL.RawQuery
	}
	http.Redirect(w, r, target, http.StatusMovedPermanently)
}

// GenerateSelfSignedCert génère un certificat auto-signé pour le développement
func GenerateSelfSignedCert(certPath string) (string, string, error) {
    certFile := filepath.Join(certPath, "localhost.crt")
    keyFile := filepath.Join(certPath, "localhost.key")
    
    // Vérifier si les fichiers existent déjà
    _, errCert := os.Stat(certFile)
    _, errKey := os.Stat(keyFile)
    
    // Si les fichiers existent déjà, les utiliser
    if errCert == nil && errKey == nil {
        log.Println("Using existing self-signed certificate:", certFile)
        return certFile, keyFile, nil
    }
    
    // Créer la commande OpenSSL pour générer le certificat
    cmd := exec.Command(
        "openssl", "req", "-x509", 
        "-newkey", "rsa:4096", 
        "-keyout", keyFile, 
        "-out", certFile, 
        "-days", "365", 
        "-nodes", 
        "-subj", "/CN=localhost",
    )
    
    output, err := cmd.CombinedOutput()
    if err != nil {
        log.Printf("Failed to generate self-signed certificate: %v\nOutput: %s", err, output)
        return "", "", err
    }
    
    log.Println("Generated new self-signed certificate:", certFile)
    return certFile, keyFile, nil
}