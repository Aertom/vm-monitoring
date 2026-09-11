# vm-monitoring backend — image minimale multi-stage (usage perso, sans auth).
# Build : docker build -t vm-monitoring-backend .
#   (contexte = racine du repo, le Dockerfile copie backend/ + exemples YAML)
# Run   : docker run --rm -p 8080:8080 \
#           -v $PWD/config.yaml:/config.yaml:ro \
#           -v $PWD/hypervisors.yaml:/hypervisors.yaml:ro \
#           vm-monitoring-backend --config /config.yaml
FROM docker.io/library/golang:1.22-bookworm AS builder
WORKDIR /src
COPY backend/go.mod backend/go.sum ./backend/
RUN cd backend && go mod download
COPY backend ./backend
RUN cd backend && CGO_ENABLED=0 go build -trimpath -o /out/vm-monitoring-server ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/vm-monitoring-server /vm-monitoring-server
COPY config.example.yaml /config.example.yaml
COPY hypervisors.example.yaml /hypervisors.example.yaml
EXPOSE 8080
ENTRYPOINT ["/vm-monitoring-server"]
CMD ["--config", "/config.yaml", "--hypervisors", "/hypervisors.yaml"]
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s \
  CMD ["/vm-monitoring-server", "--healthcheck"]
