package module

import (
	"fmt"
	"slices"
	"time"
)

// Haklar (entitlement): bir kurulumda modülün kullanabildikleri.
//
//   - Kademe (Tier): Manifest.Packages'tan biri (dar → geniş).
//   - Özellik (Feature): kademeye dahilse açık; değilse eklentiyle açılabilir (Addon).
//   - Kota (Limit): kademenin değeri; eklentiyle artırılmış son değer lisansta gelir.
//
// Modül kademe adına değil haklara bakar:
//
//	if !deps.Entitlement.Has("warehouses") { ... }
//	if !deps.Entitlement.Allows("max-branches", count) { ... }

// Unlimited, sınırsız kota değeridir.
const Unlimited = -1

// Feature, açılıp kapanabilen bir özelliktir.
type Feature struct {
	Key         string   `json:"key" doc:"Özellik anahtarı (küçük harf, rakam, tire)" example:"warehouses"`
	Name        string   `json:"name" example:"Depo kaydı"`
	Description string   `json:"description,omitempty"`
	Tiers       []string `json:"tiers,omitempty" doc:"Bu özelliği içeren kademeler"`
	Addon       bool     `json:"addon,omitempty" doc:"Kademesinde yoksa eklenti olarak satılabilir"`
}

// Limit, sayılabilir bir haktır (kota).
type Limit struct {
	Key   string         `json:"key" doc:"Kota anahtarı (küçük harf, rakam, tire)" example:"max-branches"`
	Name  string         `json:"name" example:"Müşteri başına şube"`
	Tiers map[string]int `json:"tiers" doc:"Kademe → değer (-1 sınırsız; verilmeyen kademede 0)"`
	Addon bool           `json:"addon,omitempty" doc:"Eklentiyle artırılabilir"`
}

// Entitlement, modülün bu kurulumdaki haklarıdır.
type Entitlement struct {
	Tier     string          `json:"tier,omitempty" doc:"Açık kademe (kademesiz modülde boş)"`
	Until    *time.Time      `json:"until,omitempty" doc:"Hakkın bitişi (yoksa lisans bitişine kadar)"`
	Features map[string]bool `json:"features,omitempty"`
	Limits   map[string]int  `json:"limits,omitempty"`
}

// Has, özelliğin açık olup olmadığını söyler.
func (e Entitlement) Has(feature string) bool { return e.Features[feature] }

// Limit, kotayı döner (Unlimited: sınırsız; tanımsız: 0).
func (e Entitlement) Limit(key string) int { return e.Limits[key] }

// Allows, mevcut sayı count iken bir tane daha eklenebilir mi söyler.
func (e Entitlement) Allows(key string, count int) bool {
	l := e.Limits[key]
	return l == Unlimited || count < l
}

// Over, mevcut sayı kotayı aşıyor mu söyler (plan düşürüldüğünde). Aşan kayıtlar
// silinmez: modül yenisini eklemeyi engeller, fazlasını salt okunur gösterir ve
// uyarır; yükseltilince açılır.
func (e Entitlement) Over(key string, count int) bool {
	l := e.Limits[key]
	return l != Unlimited && count > l
}

// Entitle, manifestten ve lisanstaki haktan modülün haklarını çıkarır:
// kademenin özellikleri + eklentiyle açılanlar; kademenin kotaları, lisansta
// değer verilmişse o değer (satıcı kademe + eklenti toplamını yazar).
// Manifestte tanımlı olmayan özellik/kota yok sayılır.
func Entitle(mf Manifest, tier string, features []string, limits map[string]int) Entitlement {
	e := Entitlement{Tier: tier, Features: map[string]bool{}, Limits: map[string]int{}}
	for _, f := range mf.Features {
		if slices.Contains(f.Tiers, tier) || slices.Contains(features, f.Key) {
			e.Features[f.Key] = true
		}
	}
	for _, l := range mf.Limits {
		e.Limits[l.Key] = l.Tiers[tier]
		if v, ok := limits[l.Key]; ok {
			e.Limits[l.Key] = v
		}
	}
	return e
}

// EntitleAll, geliştirme ortamı içindir: en geniş kademe, tüm özellikler, kotalar en geniş kademenin değeri.
func EntitleAll(mf Manifest) Entitlement {
	tier := ""
	if n := len(mf.Packages); n > 0 {
		tier = mf.Packages[n-1].Key
	}
	keys := make([]string, 0, len(mf.Features))
	for _, f := range mf.Features {
		keys = append(keys, f.Key)
	}
	return Entitle(mf, tier, keys, nil)
}

// TierRank, kademenin manifestteki sırasıdır (0 en dar); yoksa -1.
func TierRank(mf Manifest, tier string) int {
	return slices.IndexFunc(mf.Packages, func(p Package) bool { return p.Key == tier })
}

// ValidateEntitlements, manifestteki özellik ve kota tanımlarını denetler
// (çekirdek modülleri yüklerken çağırır).
func ValidateEntitlements(mf Manifest) error {
	tiers := map[string]bool{}
	for _, p := range mf.Packages {
		tiers[p.Key] = true
	}
	seen := map[string]bool{}
	for _, f := range mf.Features {
		if !ValidID(f.Key) || seen[f.Key] {
			return fmt.Errorf("%s: özellik anahtarı geçersiz ya da tekrar: %q", mf.ID, f.Key)
		}
		seen[f.Key] = true
		for _, t := range f.Tiers {
			if !tiers[t] {
				return fmt.Errorf("%s: %s özelliğinde tanımsız kademe %q", mf.ID, f.Key, t)
			}
		}
	}
	for _, l := range mf.Limits {
		if !ValidID(l.Key) || seen[l.Key] {
			return fmt.Errorf("%s: kota anahtarı geçersiz ya da tekrar: %q", mf.ID, l.Key)
		}
		seen[l.Key] = true
		for t, v := range l.Tiers {
			if !tiers[t] {
				return fmt.Errorf("%s: %s kotasında tanımsız kademe %q", mf.ID, l.Key, t)
			}
			if v < Unlimited {
				return fmt.Errorf("%s: %s kotası %s kademesinde geçersiz: %d", mf.ID, l.Key, t, v)
			}
		}
	}
	return nil
}
