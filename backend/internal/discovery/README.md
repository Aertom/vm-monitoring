# Packages de découverte d'hyperviseurs

Ce dossier regroupe les clients de découverte des VMs pour chaque type
d'hyperviseur supporté par vm-monitoring :

- `esxi/` — Découverte via l'API REST vSphere/ESXi.
- `ahv/` — Découverte via l'API REST Prism (Nutanix AHV).
- `kvm/` — Découverte via la commande `virsh` (libvirt), local ou distant (SSH).

## Principes de conception

Ces packages ont été développés de façon **autonome et additive** :

- **Aucune dépendance interne** (`model`, `store`, `api`) : uniquement la
  bibliothèque standard Go. Cela garantit qu'ils compilent et se testent
  indépendamment du reste de l'application, sans risque de rupture avec
  le code existant.
- **Injection de dépendances** pour toutes les I/O externes :
  - `esxi.HTTPClient` / `ahv.HTTPClient` : interface `Do(*http.Request) (*http.Response, error)`.
  - `kvm.CommandExecutor` : interface `Run(ctx, name, args...) (string, error)`.
  Cela permet un mock complet dans les tests unitaires, sans appel réseau
  ou processus réel.
- **Couverture de tests** : chaque package possède des tests couvrant le
  cas nominal, les erreurs de configuration, les erreurs de transport/exécution
  et les réponses invalides (JSON malformé, sortie texte inattendue).

## État actuel

| Package | Statut | Intégré à `api`/`store` |
|---|---|---|
| `esxi` | ✅ Fonctionnel, testé | ✅ Oui (via `internal/inventory`) |
| `ahv`  | ✅ Fonctionnel, testé | ✅ Oui (via `internal/inventory`) |
| `kvm`  | ✅ Fonctionnel, testé | ✅ Oui (via `internal/inventory`) |

Branchés dans `cmd/server/main.go` : `inventory.DiscoverAll` (parallèle,
erreurs isolées) → `inventory.Merge` (statique = référence d'identité) →
collecte SSH → `store.ReplaceVMs`. Dernier rapport sur `GET /api/discovery`.

## Notes d'intégration

- Les types bruts (`esxi.VM`, `ahv.VM`, `kvm.VM`) sont normalisés en
  `inventory.DiscoveredVM` puis fusionnés en `model.VM`.
- L'authentification TLS `insecure` est honorée par `inventory` (client HTTP
  dédié), car `esxi`/`ahv` ne la gèrent pas eux-mêmes.
- KVM distant passe par SSH avec la même clé que les VMs (`ssh.*`).
