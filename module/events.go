package module

import "fmt"

// Event, modülün bildirim olayıdır (olay yolu, Faz B). Bildirim modülü (notify) derlenmiş
// modüllerin olaylarını Deps.Registry'den okur; yeni modül için notify'a dokunulmaz.
//
// İki tür olay vardır:
//   - Anlık: denetim kaydına (Audit.Log) Action eylemi yazılınca (To verilmişse Data["to"] == To).
//     Kaydın kimliği AuditEntry.EntityID'dir.
//   - Zamanlı (Reminder): notify her turda DueSQL'i çalıştırır; dönen kimliklere bir kez mesaj gider.
//
// VarsSQL, kaydın mesaj değişkenlerini döner (tek satır, $1 = kayıt kimliği) ve sütunları sırayla:
// number, customer, phone, total (sayı metni, ör. "1250.00"), date (gösterilecek metin), time (metin).
// Boş değerler için boş metin döndürün. Tarih/saat biçimi modülde yapılır (ör. to_char(... AT TIME ZONE ...)).
//
// DueSQL parametreleri: $1 = şimdi (timestamptz), $2 = notify'ın hatırlatma ayarı (saat).
// Sorgular sadece modülün kendi tablolarını okur; notify bunları değiştirmeden çalıştırır.
type Event struct {
	Key      string `json:"key" doc:"Benzersiz anahtar (ör. order_ready)"`
	Label    string `json:"label"`
	Hint     string `json:"hint" doc:"Ne zaman gider"`
	Template string `json:"template" doc:"Önerilen metin; yer tutucular {firma} {musteri} {no} {tutar} {tarih} {saat}"`
	Reminder bool   `json:"reminder,omitempty" doc:"Zamanlı hatırlatma (notify paketinde reminders özelliği gerekir)"`

	Action  string `json:"-"` // anlık olay: denetim kaydı eylemi (ör. "orders.status")
	To      string `json:"-"` // isteğe bağlı: Data["to"] bu değerse
	VarsSQL string `json:"-"`
	DueSQL  string `json:"-"`
}

// ValidateEvents, olay tanımlarını denetler: anahtar biçimi ve tekilliği, VarsSQL, anlık olayda
// Action, zamanlı olayda DueSQL.
func ValidateEvents(mf Manifest) error {
	seen := map[string]bool{}
	for _, e := range mf.Events {
		if !eventKey(e.Key) {
			return fmt.Errorf("%s: olay anahtarı %q geçersiz (küçük harf, rakam, alt çizgi)", mf.ID, e.Key)
		}
		if seen[e.Key] {
			return fmt.Errorf("%s: olay %q iki kez tanımlı", mf.ID, e.Key)
		}
		seen[e.Key] = true
		switch {
		case e.Label == "" || e.Template == "":
			return fmt.Errorf("%s: olay %q için ad ve önerilen metin gerekli", mf.ID, e.Key)
		case e.VarsSQL == "":
			return fmt.Errorf("%s: olay %q için VarsSQL gerekli", mf.ID, e.Key)
		case e.Reminder && e.DueSQL == "":
			return fmt.Errorf("%s: zamanlı olay %q için DueSQL gerekli", mf.ID, e.Key)
		case !e.Reminder && e.Action == "":
			return fmt.Errorf("%s: anlık olay %q için Action gerekli", mf.ID, e.Key)
		}
	}
	return nil
}

func eventKey(s string) bool {
	if s == "" || len(s) > 64 {
		return false
	}
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' {
			return false
		}
	}
	return true
}
