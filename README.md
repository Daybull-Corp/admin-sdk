# admin-sdk

[admin-core](https://github.com/daybull-corp/admin-core) ile modüller arasındaki **sözleşme**. Modüller çekirdeğe değil, sadece bu pakete (ve huma, pgx) bağımlıdır.

```go
import "github.com/daybull-corp/admin-sdk/module"
```

## İçerik

| Tip | Görev |
|---|---|
| `module.Module` | Her modülün uyguladığı arayüz: `Manifest`, `Migrations`, `Register` |
| `module.Manifest` | Kimlik, yetkiler, admin menüsü, `Requires` (sözleşme sürümü), `Depends` (gereken modüller), `Settings` (ayarlar), `Packages` (satış paketleri) |
| `module.Package` | Satış paketi: anahtar, ad, özellikler. Hangisi açık → imzalı lisans (`Deps.Package`) |
| `module.Setting` | Panelden değiştirilebilen ayar: `text`, `number` (Min/Max), `bool`, `select` |
| `module.Deps` | Çekirdeğin verdiği servisler: DB, logger, `Auditor`, `Settings` (modülün kendi ayar değerleri), `Package` (lisanstaki paket) |
| `module.Protect` | Operasyonu yetkiye bağlayarak kaydeder |
| `module.Feature`, `module.Limit` | Özellik (hangi kademelerde dahil, eklentiyle satılabilir mi) ve kota (kademe başına değer, -1 sınırsız) — `Manifest.Features`, `Manifest.Limits` |
| `module.Entitlement` | Modülün bu kurulumdaki hakları (`Deps.Entitlement`): `Has(özellik)`, `Limit(kota)`, `Allows(kota, mevcutSayı)`. Modül kademe adına değil buna bakar |
| `Deps.Registry` | Derlenmiş tüm modüllerin manifestleri (satıcı modülleri: kademe sırası, özellikler) |

`licensing` paketi: kurulum lisansının biçimi (`Sign`, `Parse`, `LoadKey`). Çekirdek doğrular; satıcı tarafı (lisans aracı, kurulum modülü) imzalar. İçerik v2: `Grants` (modül → kademe, **modül başına bitiş**, eklenti özellikleri, kotalar) + `Limits` (çekirdek kotaları); imzalarken v1 alanları da doldurulur, eski çekirdekler okumaya devam eder.

Modül yazma rehberi: admin-core reposunda `docs/MODUL-YAZMA.md`.

## Sürümleme

- Geriye dönük uyumu bozan her değişiklikte `ContractVersion` artırılır ve yeni **minor** etiket atılır (`v0.x.0`).
- Modül, `Manifest.Requires` alanına `module.ContractVersion` yazar; çekirdek farklı sürümde açılmaz.

## Kurulum

Repo public; token veya ek ayar gerekmez:

```bash
go get github.com/daybull-corp/admin-sdk@latest
```
