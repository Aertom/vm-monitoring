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
| `esxi` | ✅ Fonctionnel, testé | ❌ Non |
| `ahv`  | ✅ Fonctionnel, testé | ❌ Non |
| `kvm`  | ✅ Fonctionnel, testé | ❌ Non |

Ces packages ne sont **pas encore branchés** dans le flux applicatif
principal (`cmd/server/main.go`, `internal/api`, `internal/store`). Ils
constituent la brique de collecte de données brutes par hyperviseur.

## Lot d'intégration à venir

Pour finaliser l'intégration, il faudra :

1. Convertir les types `esxi.VM`, `ahv.VM`, `kvm.VM` vers le modèle
   commun interne (`internal/model`), probablement via une interface
   `discovery.Provider` commune exposant une méthode `Discover(ctx) ([]model.VM, error)`.
2. Brancher ces providers dans le processus de collecte périodique
   (probablement orchestré depuis `cmd/server/main.go` ou un composant
   `internal/collector`).
3. Persister les résultats via `internal/store`.
4. Exposer/adapter les endpoints `internal/api` si nécessaire pour
   déclencher une découverte à la demande ou en consulter le statut.

Cette étape nécessite la connaissance exacte des signatures et structures
existantes (`model.VM`, interface du `store`, routes `api`) afin de garantir
un code strictement compatible et compilable dès le premier commit
d'intégration.
