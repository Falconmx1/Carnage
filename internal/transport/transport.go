package transport

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"io"
	"math/big"
	"net"
	"time"
)

// GenerateSelfSignedCert genera un certificado TLS autofirmado válido por 1 año.
func GenerateSelfSignedCert() (tls.Certificate, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("error generando clave RSA: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   "carnage",
			Organization: []string{"Carnage"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("error creando certificado: %w", err)
	}

	return tls.Certificate{
		Certificate: [][]byte{der},
		PrivateKey:  priv,
	}, nil
}

// ServerHandshake envuelve una conexión con TLS y deriva una clave AES-GCM.
// El Agent (víctima) usa esta función al aceptar conexiones.
func ServerHandshake(conn net.Conn, cert tls.Certificate) (net.Conn, cipher.AEAD, error) {
	tlsConn := tls.Server(conn, &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS13,
	})
	if err := tlsConn.Handshake(); err != nil {
		return nil, nil, fmt.Errorf("error en handshake TLS servidor: %w", err)
	}

	// Intercambiar secreto de sesión (32 bytes) para derivar la clave AES
	secret := make([]byte, 32)
	if _, err := io.ReadFull(tlsConn, secret); err != nil {
		return nil, nil, fmt.Errorf("error leyendo secreto de sesión: %w", err)
	}

	gcm, err := deriveGCM(secret)
	if err != nil {
		return nil, nil, err
	}

	return tlsConn, gcm, nil
}

// ClientHandshake envuelve una conexión con TLS y deriva una clave AES-GCM.
// El Admin (atacante) usa esta función al conectarse al Agent.
func ClientHandshake(conn net.Conn) (net.Conn, cipher.AEAD, error) {
	tlsConn := tls.Client(conn, &tls.Config{
		InsecureSkipVerify: true, // Necesario para certificados autofirmados en pentesting
		MinVersion:         tls.VersionTLS13,
	})
	if err := tlsConn.Handshake(); err != nil {
		return nil, nil, fmt.Errorf("error en handshake TLS cliente: %w", err)
	}

	// Generar y enviar secreto de sesión
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, nil, fmt.Errorf("error generando secreto: %w", err)
	}
	if _, err := tlsConn.Write(secret); err != nil {
		return nil, nil, fmt.Errorf("error enviando secreto: %w", err)
	}

	gcm, err := deriveGCM(secret)
	if err != nil {
		return nil, nil, err
	}

	return tlsConn, gcm, nil
}

// deriveGCM crea un AEAD AES-GCM a partir de un secreto compartido.
func deriveGCM(secret []byte) (cipher.AEAD, error) {
	key := sha256.Sum256(secret)
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("error creando cifrador AES: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("error creando GCM: %w", err)
	}
	return gcm, nil
}

// Encrypt cifra datos con AES-GCM y devuelve el nonce + ciphertext.
func Encrypt(gcm cipher.AEAD, plaintext []byte) ([]byte, error) {
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("error generando nonce: %w", err)
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt descifra datos cifrados con Encrypt.
func Decrypt(gcm cipher.AEAD, ciphertext []byte) ([]byte, error) {
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext demasiado corto")
	}
	nonce, data := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, data, nil)
}
