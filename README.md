# admin-sdk

[admin-core](https://github.com/daybull/admin-core) ile modüller arasındaki **sözleşme**. Modüller çekirdeğe değil, sadece bu pakete (ve huma, pgx) bağımlıdır.

```go
import "github.com/daybull/admin-sdk/module"
```

## İçerik

| Tip | Görev |
|---|---|
| `module.Module` | Her modülün uyguladığı arayüz: `Manifest`, `Migrations`, `Register` |
| `module.Manifest` | Kimlik, yetkiler, admin menüsü, `Requires` (sözleşme sürümü), `Depends` (gereken modüller), `Settings` (ayarlar) |
| `module.Setting` | Panelden değiştirilebilen ayar: `text`, `number` (Min/Max), `bool`, `select` |
| `module.Deps` | Çekirdeğin verdiği servisler: DB, logger, `Auditor`, `Settings` (modülün kendi ayar değerleri) |
| `module.Protect` | Operasyonu yetkiye bağlayarak kaydeder |

`licensing` paketi: kurulum lisansının biçimi (`Sign`, `Parse`, `LoadKey`). Çekirdek doğrular; satıcı tarafı (lisans aracı, kurulum modülü) imzalar.

Modül yazma rehberi: admin-core reposunda `docs/MODUL-YAZMA.md`.

## Sürümleme

- Geriye dönük uyumu bozan her değişiklikte `ContractVersion` artırılır ve yeni **minor** etiket atılır (`v0.x.0`).
- Modül, `Manifest.Requires` alanına `module.ContractVersion` yazar; çekirdek farklı sürümde açılmaz.

## Kurulum

Repo public; token veya ek ayar gerekmez:

```bash
go get github.com/daybull/admin-sdk@latest
```
