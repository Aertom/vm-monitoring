# VM Monitoring — mise en prod (backend Go + frontend nginx, via podman)

Dossier généré par `package.sh` (ne pas éditer ici : modifier `packaging/`).
Images taguées, tag courant dans `VERSION`.

Contenu de ce dossier :
- `vm-monitoring-backend.tar.gz` / `vm-monitoring-frontend.tar.gz` : images
  à charger avec `podman load` (pas besoin de rebuilder).
- `docker-compose.yml` : stack prod (images locales, aucun `build:`).
- `config.yaml` : **à renseigner** (VMs, SSH, `appDirs` par famille).
- `hypervisors.yaml` : hyperviseurs à découvrir (ESXi/AHV/KVM). La découverte
  tourne à chaque cycle : le `hostname` statique doit égaler le nom sur
  l'hyperviseur pour fusionner, sinon la VM est ajoutée (`disc-...`, sans IP).

## 0. Prérequis prod

- Linux + `podman` + `podman-compose` (`pip install podman-compose`).
- Ports hôte libres : `80` (frontend) et `8080` (backend, debug/direct).
- **Rootless** : les ports < 1024 sont interdits sans root. Soit on lance en
  root, soit on décale : `FRONTEND_PORT=8080 BACKEND_PORT=8081`.

## 1. Installer

```sh
mkdir -p /srv/vmmon && cd /srv/vmmon
# copier ici tout le contenu de delivery/
podman load -i vm-monitoring-backend.tar.gz
podman load -i vm-monitoring-frontend.tar.gz
podman images | grep vm-monitoring
```

## 2. Configurer

1. Éditez `config.yaml` : `staticVMs` (id/hostname/ip **joignables depuis
   le backend**), `ssh.user`, `appDirs` par famille.
2. Éditez `hypervisors.yaml` : vos ESXi/vCenter (`url` https, ex.
   `https://vcenter.lan`), Nutanix (`url` avec port Prism, défaut 9440),
   KVM (`host` vide = virsh local au conteneur — donc inutile en
   conteneur — sinon IP + `user`, via la même clé SSH).
   **Important** : le `hostname` statique doit égaler le nom de la VM sur
   l'hyperviseur (insensible à la casse) pour fusionner ; sinon la VM
   découverte est ajoutée sans IP (`disc-...`, visible mais non collectée).
   Fichier absent ou vide = découverte désactivée, statique seul.
2. Collecte des versions (optionnel) : copiez votre clé privée SSH dans
   `./id_ed25519` (`chmod 600`), mettez `privateKeyPath: "/ssh/key"` et
   **décommentez** le volume `./id_ed25519` dans `docker-compose.yml`.
   Par défaut la collecte est désactivée (versions `unknown`).
3. Rien à configurer côté frontend : il appelle l'API en relatif
   (`/api/...`) via le proxy nginx intégré.

## 3. Lancer

```sh
cd /srv/vmmon
podman-compose up -d
podman-compose ps
curl http://localhost:80/                    # SPA (titre VM Monitoring)
curl http://localhost:80/api/families        # ["sm","cm","ws","oa"] via proxy
curl http://localhost:8080/healthz           # {"status":"ok"} backend direct
```

Réseau sans DNS : le compose crée un réseau dédié (`172.28.99.0/24`) et
le frontend joint le backend par IP fixe (`172.28.99.10:8080`, variable
`BACKEND_UPSTREAM`) — aucun DNS requis, ni `/etc/resolv.conf` fourni.
Pour changer le sous-réseau, modifiez `subnet`, `ipv4_address` et
`BACKEND_UPSTREAM` dans `docker-compose.yml`.

Au premier démarrage, le frontend peut redémarrer une fois en attendant
que `backend` soit résolu/démarré : normal (`restart: unless-stopped`).

## 4. Points d'attention

- **État en mémoire** : checkouts et renommages sont perdus au restart
  du backend. Ne redémarrez qu'en creux, ou prévenez les utilisateurs.
- **Polling versions** : `pollIntervalSeconds` (défaut 300 ici). Statut
  `error` + `lastError` sur une VM = SSH/chemin à vérifier.
- **Logs** : `podman-compose logs backend|frontend`, suivi `-f`.
- **TLS** : exposez uniquement le frontend derrière votre reverse-proxy
  (Caddy/nginx/traefik) avec certificat ; le trafic `/api/` suit.

## 5. Mettre à jour

Nouveau `delivery/` reçu : rechargez les tars puis recréez :
```sh
podman load -i vm-monitoring-backend.tar.gz
podman load -i vm-monitoring-frontend.tar.gz
podman-compose up -d
```
`config.yaml` existant est conservé (monté en volume).

## 6. Se connecter au frontend

Depuis votre PC, ouvrez dans un navigateur :

```text
http://IP-OU-DOMAINE-DE-LA-PROD/
```

- Port `80` par défaut ; si vous avez lancé avec
  `FRONTEND_PORT=8080` : `http://IP-OU-DOMAINE-DE-LA-PROD:8080/`.
- Vous arrivez sur le dashboard : tableau des VMs (filtre par famille,
  colonne Version si la collecte SSH est active) et tableau des groupes
  (checkout/checkin, renommage au crayon).
- Tableaux vides ? Normal si `staticVMs` est vide : renseignez vos VMs
  dans `config.yaml` puis `podman-compose restart backend`.
- Rien à configurer côté navigateur : le frontend appelle l'API en
  relatif (`/api/...`) via nginx. Pour du HTTPS, placez votre
  reverse-proxy (Caddy/nginx/traefik) devant le port frontend (voir §4).

## Dépannage

- `invalid syntax` sur `if isinstance(services := ...)` au lancement :
  votre binaire `podman-compose` tourne sous Python 3.6 (trop vieux).
  Contournement : `python3 -m podman_compose up -d` (avec un python ≥ 3.8,
  vérifiez via `python3 --version`). Correctif durable : réinstallez
  podman-compose avec un Python moderne (`pip3 install -U podman-compose`).
