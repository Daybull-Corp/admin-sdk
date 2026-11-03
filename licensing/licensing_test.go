package licensing

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var now = time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)

func sample() License {
	return License{Customer: "Acme", Modules: []string{"customers", "notes"}, Packages: map[string]string{"customers": "kurumsal"},
		IssuedAt: now, ExpiresAt: now.AddDate(1, 0, 0)}
}

func TestSignParse(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	tok, err := Sign(priv, sample())
	if err != nil {
		t.Fatal(err)
	}
	l, err := Parse(tok, pub, now)
	if err != nil || l.Customer != "Acme" || l.Packages["customers"] != "kurumsal" {
		t.Fatalf("%+v %v", l, err)
	}
	if _, err := Parse(tok, pub, now.AddDate(2, 0, 0)); !errors.Is(err, ErrExpired) {
		t.Fatalf("süresi dolmuş bekleniyordu: %v", err)
	}
	other, _, _ := ed25519.GenerateKey(rand.Reader)
	if _, err := Parse(tok, other, now); !errors.Is(err, ErrSignature) {
		t.Fatalf("imza hatası bekleniyordu: %v", err)
	}
	if _, err := Parse("v2.x.y", pub, now); !errors.Is(err, ErrFormat) {
		t.Fatalf("biçim hatası bekleniyordu: %v", err)
	}
}

func TestSignValidates(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	bad := []func(*License){
		func(l *License) { l.Customer = "" },
		func(l *License) { l.ExpiresAt = l.IssuedAt },
		func(l *License) { l.Modules = []string{"Müşteri"} },
		func(l *License) { l.Packages = map[string]string{"blog": "temel"} },
		func(l *License) { l.Packages = map[string]string{"customers": "Kurumsal!"} },
	}
	for i, f := range bad {
		l := sample()
		f(&l)
		if _, err := Sign(priv, l); err == nil {
			t.Errorf("%d: hata bekleniyordu", i)
		}
	}
}

func TestLoadKey(t *testing.T) {
	seed := make([]byte, ed25519.SeedSize)
	_, _ = rand.Read(seed)
	p := filepath.Join(t.TempDir(), "imza.key")
	_ = os.WriteFile(p, []byte(base64.StdEncoding.EncodeToString(seed)+"\n"), 0o600)
	k, err := LoadKey(p)
	if err != nil || !k.Equal(ed25519.NewKeyFromSeed(seed)) {
		t.Fatalf("anahtar okunamadı: %v", err)
	}
	_ = os.WriteFile(p, []byte("bozuk"), 0o600)
	if _, err := LoadKey(p); err == nil {
		t.Fatal("bozuk anahtar reddedilmeli")
	}
}

func TestGrantsV2(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	addonEnd := now.AddDate(0, 1, 0)
	l := License{Customer: "Acme", IssuedAt: now, ExpiresAt: now.AddDate(1, 0, 0),
		Grants: map[string]Grant{
			"customers": {Tier: "standart", Features: []string{"warehouses"}, Limits: map[string]int{"max-branches": 8}},
			"e-invoice": {Until: &addonEnd},
		},
		Limits: map[string]int{"users": 10},
	}
	tok, err := Sign(priv, l)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Parse(tok, pub, now)
	if err != nil {
		t.Fatal(err)
	}
	// Eski çekirdekler için v1 alanları doldurulur.
	if len(got.Modules) != 2 || got.Modules[0] != "customers" || got.Packages["customers"] != "standart" {
		t.Fatalf("v1 alanları: %+v %+v", got.Modules, got.Packages)
	}
	g, ok := got.Grant("customers")
	if !ok || g.Tier != "standart" || g.Features[0] != "warehouses" || g.Limits["max-branches"] != 8 {
		t.Fatalf("hak: %+v", g)
	}
	inv, _ := got.Grant("e-invoice")
	if !inv.ActiveAt(now) || inv.ActiveAt(addonEnd) {
		t.Fatal("eklenti bitişi modül bazında olmalı")
	}
	if _, ok := got.Grant("notes"); ok {
		t.Fatal("lisansta olmayan modülün hakkı olmamalı")
	}

	late := now.AddDate(2, 0, 0)
	bad := l
	bad.Grants = map[string]Grant{"customers": {Until: &late}}
	if _, err := Sign(priv, bad); err == nil {
		t.Fatal("lisans bitişinden sonraki hak reddedilmeli")
	}
	bad.Grants = map[string]Grant{"customers": {Limits: map[string]int{"x": -5}}}
	if _, err := Sign(priv, bad); err == nil {
		t.Fatal("geçersiz kota reddedilmeli")
	}
}

func TestGrantV1(t *testing.T) {
	l := sample()
	g, ok := l.Grant("customers")
	if !ok || g.Tier != "kurumsal" || !g.ActiveAt(now.AddDate(5, 0, 0)) {
		t.Fatalf("v1 hak: %+v %v", g, ok)
	}
	if g, ok := l.Grant("notes"); !ok || g.Tier != "" {
		t.Fatalf("v1 kademesiz: %+v %v", g, ok)
	}
}
