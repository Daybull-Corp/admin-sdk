package module

import "testing"

func sampleManifest() Manifest {
	return Manifest{ID: "customers",
		Packages: []Package{{Key: "temel"}, {Key: "standart"}, {Key: "kurumsal"}},
		Features: []Feature{
			{Key: "branches", Tiers: []string{"standart", "kurumsal"}, Addon: true},
			{Key: "warehouses", Tiers: []string{"kurumsal"}, Addon: true},
		},
		Limits: []Limit{{Key: "max-branches", Tiers: map[string]int{"temel": 0, "standart": 5, "kurumsal": Unlimited}, Addon: true}},
	}
}

func TestEntitle(t *testing.T) {
	mf := sampleManifest()
	e := Entitle(mf, "temel", nil, nil)
	if e.Has("branches") || e.Allows("max-branches", 0) {
		t.Fatalf("temel: %+v", e)
	}
	// Temel + şube eklentisi: özellik açılır, kota lisanstan gelir.
	e = Entitle(mf, "temel", []string{"branches", "olmayan"}, map[string]int{"max-branches": 3, "olmayan": 9})
	if !e.Has("branches") || e.Has("olmayan") || !e.Allows("max-branches", 2) || e.Allows("max-branches", 3) {
		t.Fatalf("eklenti: %+v", e)
	}
	if _, ok := e.Limits["olmayan"]; ok {
		t.Fatal("tanımsız kota yok sayılmalı")
	}
	e = Entitle(mf, "kurumsal", nil, nil)
	if !e.Has("warehouses") || !e.Allows("max-branches", 1_000_000) {
		t.Fatalf("kurumsal: %+v", e)
	}
	all := EntitleAll(mf)
	if all.Tier != "kurumsal" || !all.Has("branches") || !all.Has("warehouses") {
		t.Fatalf("geliştirme: %+v", all)
	}
	if TierRank(mf, "standart") != 1 || TierRank(mf, "yok") != -1 {
		t.Fatal("kademe sırası")
	}
}

func TestValidateEntitlements(t *testing.T) {
	if err := ValidateEntitlements(sampleManifest()); err != nil {
		t.Fatal(err)
	}
	mf := sampleManifest()
	mf.Features = append(mf.Features, Feature{Key: "x", Tiers: []string{"altin"}})
	if ValidateEntitlements(mf) == nil {
		t.Fatal("tanımsız kademe reddedilmeli")
	}
	mf = sampleManifest()
	mf.Limits = append(mf.Limits, Limit{Key: "branches", Tiers: map[string]int{}})
	if ValidateEntitlements(mf) == nil {
		t.Fatal("özellik ile aynı adlı kota reddedilmeli")
	}
}

func TestOver(t *testing.T) {
	e := Entitlement{Limits: map[string]int{"branches": 3, "users": Unlimited}}
	if e.Over("branches", 3) || !e.Over("branches", 4) || e.Over("users", 1_000_000) || !e.Over("tanimsiz", 1) || e.Over("tanimsiz", 0) {
		t.Fatal("Over hatalı")
	}
}
