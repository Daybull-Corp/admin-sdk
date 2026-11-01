// Package licensing, kurulum lisansının biçimidir: imzalama (satıcı tarafı:
// lisans aracı, kurulum modülü) ve doğrulama (çekirdek). Biçim tek yerde durur
// ki iki taraf ayrışmasın.
//
// Biçim: "v1.<içerik>.<imza>" — içerik JSON'un base64url hali, imza "v1.<içerik>"
// metninin Ed25519 imzasıdır. İmzalama anahtarı sadece satıcıda durur; doğrulama
// anahtarı çekirdeğe gömülüdür.
//
// İçerik sürümleri (zarf "v1." aynı kalır, alanlar geriye uyumlu eklenir):
//
//	1: Modules + Packages (modül → kademe), tek bitiş (ExpiresAt)
//	2: Grants: modül başına kademe, bitiş, eklenti özellikleri, kotalar; Limits: çekirdek kotaları.
//	   İmzalarken Modules/Packages Grants'tan doldurulur (eski çekirdekler okumaya devam eder);
//	   ExpiresAt en geç biten hakkın bitişidir.
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

	"github.com/daybull-corp/admin-sdk/module"
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
	// Grants (v2), modül → hak. Varsa Modules/Packages yerine bu okunur.
	Grants map[string]Grant `json:"grants,omitempty"`
	// Limits (v2), çekirdek kotaları (ör. "users").
	Limits map[string]int `json:"limits,omitempty"`
}

// Grant, bir modülün lisanstaki hakkıdır.
type Grant struct {
	Tier     string         `json:"tier,omitempty" doc:"Kademe (boş: en dar ya da kademesiz)"`
	Until    *time.Time     `json:"until,omitempty" doc:"Bu hakkın bitişi (yoksa lisans bitişi)"`
	Features []string       `json:"features,omitempty" doc:"Eklentiyle açılan özellikler (kademeye dahil olanlar yazılmaz)"`
	Limits   map[string]int `json:"limits,omitempty" doc:"Kota son değerleri (kademe + eklenti; -1 sınırsız)"`
}

// ActiveAt, hakkın verilen anda geçerli olup olmadığını söyler (lisans bitişi ayrıca denetlenir).
func (g Grant) ActiveAt(t time.Time) bool { return g.Until == nil || t.Before(*g.Until) }

// Grant, modülün hakkını döner. v1 lisansta Modules/Packages'tan üretilir.
func (l License) Grant(id string) (Grant, bool) {
	if l.Grants != nil {
		g, ok := l.Grants[id]
		return g, ok
	}
	if !slices.Contains(l.Modules, id) {
		return Grant{}, false
	}
	return Grant{Tier: l.Packages[id]}, true
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
	if l.Grants != nil {
		if err := l.fromGrants(); err != nil {
			return "", err
		}
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

// fromGrants, v2 haklarını denetler ve v1 alanlarını (Modules, Packages) doldurur.
func (l *License) fromGrants() error {
	l.Modules = make([]string, 0, len(l.Grants))
	l.Packages = map[string]string{}
	for id, g := range l.Grants {
		if !module.ValidID(id) {
			return fmt.Errorf("modül kimliği geçersiz: %q", id)
		}
		if g.Tier != "" {
			if !module.ValidID(g.Tier) {
				return fmt.Errorf("%s modülünün kademesi geçersiz: %q", id, g.Tier)
			}
			l.Packages[id] = g.Tier
		}
		if g.Until != nil && g.Until.After(l.ExpiresAt) {
			return fmt.Errorf("%s modülünün bitişi lisans bitişinden sonra olamaz", id)
		}
		for _, f := range g.Features {
			if !module.ValidID(f) {
				return fmt.Errorf("%s modülünde özellik anahtarı geçersiz: %q", id, f)
			}
		}
		for k, v := range g.Limits {
			if !module.ValidID(k) || v < module.Unlimited {
				return fmt.Errorf("%s modülünde kota geçersiz: %q = %d", id, k, v)
			}
		}
		l.Modules = append(l.Modules, id)
	}
	slices.Sort(l.Modules)
	if len(l.Packages) == 0 {
		l.Packages = nil
	}
	for k, v := range l.Limits {
		if !module.ValidID(k) || v < module.Unlimited {
			return fmt.Errorf("çekirdek kotası geçersiz: %q = %d", k, v)
		}
	}
	return nil
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
