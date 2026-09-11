# Frontend vm-monitoring — build statique React servi par nginx.
# Build args :
#   REACT_APP_API_URL : URL de l'API vue par le navigateur.
#     - "same-origin" (défaut) : appels relatifs /api/... via le proxy nginx
#       du docker-compose (aucune config navigateur requise).
#     - sinon URL absolue, ex https://api.ton-domaine.fr (GitHub Pages, nginx seul).
#   PUBLIC_URL : préfixe des assets ("/" par défaut ; "/vm-monitoring/" pour Pages).
# Exemple compose/prod : podman build --isolation=chroot --format=docker \
#   --build-arg REACT_APP_API_URL=same-origin -t vm-monitoring-frontend .
FROM docker.io/library/node:18-bookworm AS builder
WORKDIR /src
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
ARG REACT_APP_API_URL=same-origin
ARG PUBLIC_URL=/
ENV REACT_APP_API_URL=$REACT_APP_API_URL PUBLIC_URL=$PUBLIC_URL
RUN npm run build

FROM docker.io/library/nginx:1.27-alpine
# Template substitué au démarrage (BACKEND_UPSTREAM) : aucun DNS requis.
ENV BACKEND_UPSTREAM=backend:8080
COPY nginx.conf.template /etc/nginx/templates/default.conf.template
COPY --from=builder /src/build /usr/share/nginx/html
EXPOSE 80
