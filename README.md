# VM Monitoring

A comprehensive web-based application for managing hypervisor VM groups and their checkout/checkin operations. Built with a Go backend API and a React/TypeScript frontend, deployed statically on GitHub Pages.

## Features

- **VM Inventory**: View all virtual machines across hypervisors (static inventory from YAML config)
- **Group Management**: Automatic group reconstruction from `/etc/hosts` cross-references between VMs
- **Family Detection**: Automatic classification of VMs into families (sm, cm, ws, oa) based on hostname patterns
- **Checkout/Checkin**: Atomic operations to reserve VM groups for exclusive use
- **Real-time Dashboard**: Responsive web UI with family filtering and status indicators

## Architecture

### Backend (Go)

- **Type System**: Unified data models (`VM`, `Group`, `Family`, `Status`, `Hypervisor`)
- **Store**: Thread-safe in-memory store with persistent checkout metadata
- **API**: RESTful HTTP API (chi router) on port 8080
- **Config**: YAML-based configuration for inventory and hypervisors
- **Grouping Logic**: Union-find algorithm to reconstruct groups from `/etc/hosts` cross-references

### Frontend (React + TypeScript)

- **Build**: Static SPA deployable on GitHub Pages via `npm run build`
- **Components**: 
  - `VMTable`: Filterable table of VMs by family
  - `GroupTable`: Group management with checkout/checkin actions
  - `App`: Central coordination and API consumption
- **API Client**: Axios-based client with type safety
- **Styles**: CSS modules for scoped styling

### Deployment

- **CI/CD**: GitHub Actions workflows for backend tests, frontend build, and Pages deployment
- **Hosting**: Frontend served from GitHub Pages
- **API**: Backend runs independently (e.g., cloud VM, on-premises server)

## Local Development

### Prerequisites

- **Go 1.21+** for backend
- **Node.js 18+** for frontend
- **npm** or compatible package manager

### Backend Setup

```bash
cd backend

# Download dependencies
go mod download

# Build
go build -v ./...

# Run tests
go test -v ./...

# Start server
go run ./cmd/server/main.go
```

The API will be available at `http://localhost:8080`.

#### Configuration

Create `config.yaml` at the repository root (or specify via env var `CONFIG_FILE`):

```yaml
vms:
  - id: "vm-sm-01"
    hostname: "sm-prod-01"
    ip: "192.168.1.10"
    hypervisor: "esx-01"
  - id: "vm-cm-01"
    hostname: "cm-prod-01"
    ip: "192.168.1.20"
    hypervisor: "esx-01"
  - id: "vm-ws-01"
    hostname: "ws-prod-01"
    ip: "192.168.1.30"
    hypervisor: "esx-02"
  - id: "vm-oa-01"
    hostname: "oa-prod-01"
    ip: "192.168.1.40"
    hypervisor: "esx-02"

hypervisors:
  esx-01:
    type: "esxi"
    host: "192.168.0.10"
    username: "root"
    password: "password"
  esx-02:
    type: "esxi"
    host: "192.168.0.11"
    username: "root"
    password: "password"
```

See `config.example.yaml` and `hypervisors.example.yaml` for complete examples.

### Frontend Setup

```bash
cd frontend

# Install dependencies
npm install

# Start development server (http://localhost:3000)
npm start

# Build for production
npm run build

# Run tests
npm test
```

#### Environment Variables

Create `.env.local`:

```
REACT_APP_API_URL=http://localhost:8080
```

Or for GitHub Pages deployment, set in workflow:

```
REACT_APP_API_URL=https://api.example.com
```

## API Endpoints

### GET /api/vms

Fetch all VMs, optionally filtered by family.

**Query Parameters:**
- `family` (optional): Filter by family (sm, cm, ws, oa)

**Response:**
```json
[
  {
    "id": "vm-sm-01",
    "hostname": "sm-prod-01",
    "ip": "192.168.1.10",
    "family": "sm",
    "status": "available",
    "inUseBy": null,
    "checkedOutAt": null
  }
]
```

### GET /api/groups

Fetch all groups reconstructed from VM cross-references.

**Response:**
```json
[
  {
    "id": "group-1",
    "vms": [...],
    "status": "available",
    "inUseBy": null,
    "checkedOutAt": null
  }
]
```

### GET /api/families

Fetch all detected VM families.

**Response:**
```json
["sm", "cm", "ws", "oa"]
```

### POST /api/groups/{id}/checkout

Reserve a group for exclusive use.

**Request Body:**
```json
{
  "inUseBy": "john.doe"
}
```

**Response:**
```json
{
  "id": "group-1",
  "vms": [...],
  "status": "checkedOut",
  "inUseBy": "john.doe",
  "checkedOutAt": "2026-09-10T08:14:00Z"
}
```

### POST /api/groups/{id}/checkin

Release a group back to available state.

**Response:**
```json
{
  "id": "group-1",
  "vms": [...],
  "status": "available",
  "inUseBy": null,
  "checkedOutAt": null
}
```

## Technical Decisions

### VM Family Discovery

Families are detected via substring matching (case-insensitive) on the VM hostname:
- `sm`: Storage/Memory node (e.g., "sm-prod-01")
- `cm`: Compute node (e.g., "cm-prod-01")
- `ws`: Workstation (e.g., "ws-prod-01")
- `oa`: Orchestration Agent (e.g., "oa-prod-01")
- `unknown`: No match

### Group Reconstruction from /etc/hosts

1. **Collection**: For each non-OA VM, SSH into it and read `/etc/hosts`
2. **Parsing**: Extract all IP addresses and match them to known VMs in the inventory
3. **Union-Find**: Build groups by treating VMs as nodes; two VMs are in the same group if they reference each other in `/etc/hosts`
4. **OA Attachment**: VMs of family "oa" are attached to all groups that reference them
5. **Isolation**: VMs without cross-references form single-member groups

**Rationale**: OA VMs are shared orchestration services; non-OA families (sm, cm, ws) form logical application stacks.

### Store & Checkout/Checkin

- **Thread-safe**: Go mutex protects the in-memory store
- **Persistent Metadata**: `InUseBy` and `CheckedOutAt` survive group reconstruction cycles
- **Atomic Operations**: Checkout and checkin are single transactions
- **No Persistence Layer**: Data is kept in memory; use external state management (Redis, DB) for multi-replica deployments

## Testing

### Backend Unit Tests

```bash
cd backend
go test -v ./...
```

Coverage includes:
- `store_test.go`: Store operations (Checkout, Checkin, thread safety)
- `config_test.go`: YAML config parsing
- `grouping_test.go`: Group reconstruction from mock `/etc/hosts` data
- `family_test.go`: Family detection from hostname patterns

### Frontend Tests

```bash
cd frontend
npm test
```

React Testing Library tests for components and integration scenarios.

## Deployment

### GitHub Pages

The frontend is automatically deployed to GitHub Pages on every push to `main`:

1. **Workflow**: `.github/workflows/deploy-pages.yml`
2. **Artifact**: Static build output (`frontend/build/`)
3. **URL**: `https://Aertom.github.io/vm-monitoring/`

Update the workflow's `cname` field if using a custom domain.

### Backend Deployment

Deploy the backend independently:

```bash
cd backend
go build -o vm-monitoring-server ./cmd/server
./vm-monitoring-server
```

Or containerize:

```dockerfile
FROM golang:1.21 AS builder
WORKDIR /app
COPY backend /app
RUN go build -o vm-monitoring-server ./cmd/server

FROM gcr.io/distroless/base-debian11
COPY --from=builder /app/vm-monitoring-server /
COPY config.yaml /
CMD ["/vm-monitoring-server"]
```

## Project Structure

```
vm-monitoring/
├── backend/
│   ├── cmd/
│   │   └── server/
│   │       └── main.go
│   ├── internal/
│   │   ├── api/
│   │   │   └── api.go
│   │   ├── config/
│   │   │   ├── config.go
│   │   │   └── config_test.go
│   │   ├── grouping/
│   │   │   ├── grouping.go
│   │   │   └── grouping_test.go
│   │   ├── model/
│   │   │   ├── vm.go
│   │   │   ├── family.go
│   │   │   └── family_test.go
│   │   └── store/
│   │       ├── store.go
│   │       └── store_test.go
│   ├── go.mod
│   └── go.sum
├── frontend/
│   ├── public/
│   │   └── index.html
│   ├── src/
│   │   ├── api/
│   │   │   └── client.ts
│   │   ├── components/
│   │   │   ├── GroupTable.tsx
│   │   │   ├── GroupTable.css
│   │   │   ├── VMTable.tsx
│   │   │   └── VMTable.css
│   │   ├── types/
│   │   │   └── index.ts
│   │   ├── App.tsx
│   │   ├── App.css
│   │   ├── index.tsx
│   │   └── index.css
│   ├── package.json
│   ├── package-lock.json
│   ├── tsconfig.json
│   └── .env.example
├── .github/
│   └── workflows/
│       ├── backend-ci.yml
│       ├── frontend-ci.yml
│       └── deploy-pages.yml
├── config.example.yaml
├── hypervisors.example.yaml
└── README.md
```

## Next Steps

- [ ] Integrate actual ESXi/AHV/KVM discovery (SSH `virsh` queries, vSphere API calls)
- [ ] Add persistent state layer (Redis, PostgreSQL)
- [ ] Implement multi-replica backend with consensus (Raft, etcd)
- [ ] Frontend: Add user authentication and audit logging
- [ ] Backend: Add rate limiting and request validation
- [ ] Monitoring: Prometheus metrics, health checks

## License

MIT
