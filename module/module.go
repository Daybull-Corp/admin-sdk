// Package module, çekirdek ile eklenti modülleri arasındaki ortak sözleşmedir.
//
// Ayrı repolardaki modüller sadece bu paketi (ve huma, pgx) içe aktarır;
// çekirdeğin internal paketlerine erişmez. Bu paket değiştiğinde tüm
// modüller etkilenir; geriye dönük uyumu bozan değişikliklerde
// ContractVersion artırılır.
//
// Sürüm geçmişi:
//
//	1: çekirdek içinde pkg/module
//	2: ayrı repo github.com/daybull/admin-sdk/module
//	3: Manifest.Depends (modül bağımlılığı), Manifest.Settings + Deps.Settings (modül ayarları)
//	   + (geriye uyumlu) Manifest.Packages + Deps.Package (satış paketleri, lisansla seçilir)
package module

import (
	"context"
	"io/fs"
	"log/slog"
	"regexp"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ContractVersion, modül sözleşmesinin sürümüdür. Modül, Manifest.Requires
// alanında desteklediği sürümü belirtir; uyuşmazsa çekirdek açılmaz.
const ContractVersion = 3

// Module, her eklentinin uygulaması gereken arayüzdür.
type Module interface {
	// Manifest modülün kimliğini, yetkilerini ve admin menüsünü tanımlar.
	Manifest() Manifest
	// Migrations, modülün SQL migration dosyalarını döner (goose formatı).
	// Tablosu olmayan modül nil dönebilir.
	Migrations() fs.FS
	// Register, modülün API uç noktalarını kaydeder. api, "/api/v1/<id>"
	// önekine sahip bir gruptur; modüller başka bir yola kayıt yapamaz.
	Register(api huma.API, deps Deps) error
}

// Manifest, modülün kendini tanıttığı bilgilerdir. Admin paneli menüsünü
// ve yetki ekranlarını bu bilgilerden oluşturur.
type Manifest struct {
	ID          string       `json:"id" doc:"Benzersiz modül kimliği (küçük harf, rakam, tire)" example:"notes"`
	Name        string       `json:"name" doc:"Görünen ad" example:"Notlar"`
	Version     string       `json:"version" doc:"Modül sürümü (semver)" example:"1.0.0"`
	Description string       `json:"description,omitempty"`
	Requires    int          `json:"requires" doc:"Desteklenen sözleşme sürümü"`
	Permissions []Permission `json:"permissions"`
	Menu        []MenuItem   `json:"menu"`
	// Depends, bu modülün çalışması için açık olması gereken modüllerdir.
	// Çekirdek: bağımlılık kurulumda yoksa ya da döngü varsa açılmaz;
	// bağımlılığın lisansı yoksa bu modül de kapalı kalır. Migration'lar
	// bağımlılıktan sonra çalışır (bağımlılığın tablosuna FK verilebilir).
	Depends []string `json:"depends,omitempty" doc:"Bu modülün çalışması için gereken modüller"`
	// Settings, müşteri bazında panelden değiştirilebilen ayarlardır.
	Settings []Setting `json:"settings,omitempty" doc:"Modül ayarları (panelde form olarak gösterilir)"`
	// Packages, modülün satış paketleridir (dar → geniş sırayla). Hangisinin açık
	// olduğunu imzalı lisans söyler; lisans paket belirtmezse ilki (en dar),
	// geliştirmede lisans yoksa sonuncusu (en geniş) kullanılır. Boşsa modül paketsizdir.
	Packages []Package `json:"packages,omitempty" doc:"Satış paketleri (dar → geniş)"`
}

// Package, modülün bir satış paketidir. Features, paketin açtığı özellik ve
// sınırlardır (modüle özgü anahtarlar; panel karşılaştırma tablosu için).
// Modül, Deps.Package.Key'e göre kendi kurallarını uygular.
type Package struct {
	Key         string         `json:"key" doc:"Paket anahtarı (küçük harf, rakam, tire)" example:"kurumsal"`
	Name        string         `json:"name" example:"Kurumsal"`
	Description string         `json:"description,omitempty"`
	Features    map[string]any `json:"features,omitempty" doc:"Özellik → değer (bool ya da sayı)"`
}

// Permission, modülün tanımladığı bir yetkidir. Key "<modül-id>." ile başlamalıdır.
type Permission struct {
	Key         string `json:"key" example:"notes.read"`
	Description string `json:"description" example:"Notları görüntüleme"`
}

// MenuItem, admin panelindeki bir menü öğesidir.
type MenuItem struct {
	Label      string `json:"label" example:"Notlar"`
	Path       string `json:"path" doc:"Admin panelindeki sayfa yolu" example:"/notes"`
	Icon       string `json:"icon,omitempty" doc:"İkon adı (panel tarafında eşlenir)" example:"notebook"`
	Permission string `json:"permission,omitempty" doc:"Menünün görünmesi için gereken yetki" example:"notes.read"`
}

// Deps, çekirdeğin modüllere sağladığı ortak servislerdir.
type Deps struct {
	DB       *pgxpool.Pool
	Log      *slog.Logger
	Audit    Auditor
	Settings Settings // modülün kendi ayarları (Manifest.Settings)
	Package  Package  // lisansın açtığı paket (Manifest.Packages'tan biri; paketsiz modülde boş)
}

// Auditor, denetim kaydı yazar. Kullanıcı ve IP bilgisi ctx'ten otomatik alınır.
type Auditor interface {
	Log(ctx context.Context, e AuditEntry)
}

// AuditEntry, tek bir denetim kaydıdır.
type AuditEntry struct {
	Action   string         // örn. "notes.create"
	Entity   string         // örn. "note"
	EntityID string         // örn. "42"
	Data     map[string]any // isteğe bağlı ek bilgi (gizli veri koymayın)
}

var idPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{1,31}$`)

// ValidID, modül kimliğinin geçerli olup olmadığını söyler.
func ValidID(id string) bool { return idPattern.MatchString(id) }
