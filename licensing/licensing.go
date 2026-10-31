// Package licensing, kurulum lisansının biçimidir: imzalama (satıcı tarafı:
// lisans aracı, kurulum modülü) ve doğrulama (çekirdek). Biçim tek yerde durur
// ki iki taraf ayrışmasın.
//
// Biçim: "v1.<içerik>.<imza>" — içerik JSON'un base64url hali, imza "v1.<içerik>"
// metninin Ed25519 imzasıdır. İmzalama anahtarı sadece satıcıda durur; doğrulama
// anahtarı çekirdeğe gömülüdür.
package licensing

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/daybull/admin-sdk/module"
)

const prefix = "v1."

// License, imzalı lisansın içeriğidir.
type License struct {
	Customer  string    `json:"customer"`
	Domain    string    `json:"domain,omitempty"`
	Modules   []string  `json:"modules"`
	IssuedAt  time.Time `json:"issued_at"`
	ExpiresAt time.Time `json:"expires_at"`
	// Packages, modül → paket anahtarı. Verilmeyen modül en dar paketiyle açılır.
	Packages map[string]string `json:"packages,omitempty"`
}

var (
	ErrFormat    = errors.New("lisans biçimi geçersiz")
	ErrSignature = errors.New("lisans imzası geçersiz")
	ErrExpired   = errors.New("lisansın süresi dolmuş")
)

// Sign, lisansı doğrular, imzalar ve tek satırlık metin olarak döner.
func Sign(priv ed25519.PrivateKey, l License) (string, error) {
	if l.Customer == "" {
		return "", errors.New("müşteri adı boş olamaz")
	}
	if !l.ExpiresAt.After(l.IssuedAt) {
		return "", errors.New("bitiş tarihi veriliş tarihinden sonra olmalı")
	}
	for _, id := range l.Modules {
		if !module.ValidID(id) {
			return "", fmt.Errorf("modül kimliği geçersiz: %q", id)
		}
	}
	for id, pkg := range l.Packages {
		if !slices.Contains(l.Modules, id) {
			return "", fmt.Errorf("%s modülüne paket verilmiş ama modül lisansta yok", id)
		}
		if !module.ValidID(pkg) {
			return "", fmt.Errorf("%s modülünün paket anahtarı geçersiz: %q", id, pkg)
		}
	}
	payload, err := json.Marshal(l)
	if err != nil {
		return "", err
	}
	signed := prefix + base64.RawURLEncoding.EncodeToString(payload)
	sig := ed25519.Sign(priv, []byte(signed))
	return signed + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

// Parse, imzayı ve süreyi doğrular. Süresi dolmuşsa içerikle birlikte ErrExpired döner.
func Parse(token string, pub ed25519.PublicKey, now time.Time) (License, error) {
	var l License
	token = strings.TrimSpace(token)
	if !strings.HasPrefix(token, prefix) {
		return l, ErrFormat
	}
	i := strings.LastIndexByte(token, '.')
	if i <= len(prefix) {
		return l, ErrFormat
	}
	signed, sigPart := token[:i], token[i+1:]
	sig, err := base64.RawURLEncoding.DecodeString(sigPart)
	if err != nil {
		return l, ErrFormat
	}
	if len(pub) != ed25519.PublicKeySize || !ed25519.Verify(pub, []byte(signed), sig) {
		return l, ErrSignature
	}
	payload, err := base64.RawURLEncoding.DecodeString(signed[len(prefix):])
	if err != nil {
		return l, ErrFormat
	}
	if err := json.Unmarshal(payload, &l); err != nil {
		return l, ErrFormat
	}
	if !now.Before(l.ExpiresAt) {
		return l, ErrExpired
	}
	return l, nil
}

// LoadKey, lisans aracının ürettiği anahtar dosyasını (base64 tohum) okur.
func LoadKey(path string) (ed25519.PrivateKey, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	seed, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil || len(seed) != ed25519.SeedSize {
		return nil, errors.New("imzalama anahtarı okunamadı")
	}
	return ed25519.NewKeyFromSeed(seed), nil
}
