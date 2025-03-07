# Task Master - Scalable Job Processing System

A background job processing system that handles heavy or time-consuming tasks—like sending emails, processing images, or automating workflows—so your main app stays fast and responsive.

## 🚀 Quick Start

```bash
# Start development environment
docker compose -f docker-compose.dev.yml up --build

# Run database migrations
docker exec -i postgres-db psql -U admin -d taskmaster < db/schema.sql


#connect to postgres cli
docker exec -it postgres-db psql -U admin -d taskmaster


# Run tests
go test ./... -v
```

## 📋 Implementation Progress

### 1. Core Infrastructure ✅

- [x] Project structure and module setup
- [x] PostgreSQL database integration
- [x] Redis for state management
- [x] Kafka message queue setup
- [x] Docker containerization
- [x] Basic authentication (JWT)
- [x] Configuration management
- [ ] gRPC API implementation

### 2. Job Processing System 🚧

#### Basic Features ✅

- [x] REST API endpoints (Fiber)
- [x] Job creation and storage
- [x] Kafka producer implementation
- [x] Worker service setup
- [x] Basic job status tracking
- [x] Simple retry mechanism

#### Core Processing Features 🚧

- [ ] Job type registry system
- [ ] Payload validation
- [ ] Configurable retry policies
- [ ] Job timeout handling
- [ ] Progress tracking
- [ ] Job dependencies
- [ ] Job cancellation
- [ ] Priority queues
- [ ] Dead-letter queue
- [ ] Result storage

#### Advanced Processing Features 🚧

- [ ] Distributed locking
- [ ] Job batching
- [ ] Workflow engine
- [ ] Cron scheduling
- [ ] Rate limiting
- [ ] Job routing
- [ ] Job chaining
- [ ] Recovery system

### 3. Developer Experience 🚧

#### Documentation & Tools

- [ ] Job type documentation
- [ ] Debugging tools
- [ ] Testing/simulation tools
- [ ] Job templates
- [ ] Payload transformation
- [ ] Hooks/middleware system

#### Web Dashboard

- [ ] React + Tailwind UI
- [ ] Real-time updates (WebSocket)
- [ ] Job filtering and search
- [ ] Analytics dashboard
- [ ] Retry controls
- [ ] Logs viewer
- [ ] Worker management

### 4. Operations & Monitoring 🚧

#### Observability

- [x] Structured logging (slog)
- [ ] Prometheus metrics
- [ ] Grafana dashboards
- [ ] OpenTelemetry tracing
- [ ] Health checks
- [ ] Resource monitoring

#### Operational Tools

- [ ] Job archival
- [ ] Cleanup policies
- [ ] Audit logging
- [ ] Cost tracking
- [ ] Quota management
- [ ] Statistics collection
- [ ] Alerts/notifications

### 5. Production Deployment 🚧

#### Infrastructure

- [ ] Kubernetes setup
- [ ] Helm charts
- [ ] Worker auto-scaling
- [ ] Multi-region support
- [ ] Backup/restore system
- [ ] Blue-green deployments

#### Cloud Integration

- [ ] Terraform configurations
- [ ] AWS/GCP deployment
- [ ] Cost optimization
- [ ] Security hardening

## 🔄 System Architecture

### Components

1. **API Service**

   - REST/gRPC endpoints
   - Request validation
   - Job creation & queuing

2. **Message Queue**

   - Kafka-based processing
   - Job distribution
   - Order guarantee

3. **Worker Service**

   - Job execution
   - Status management
   - Error handling

4. **Storage Layer**
   - PostgreSQL: Job data
   - Redis: Status cache
   - Kafka: Message queue

### Basic Job Flow

1. Submit job via API
2. Store in PostgreSQL
3. Queue in Kafka
4. Process via Worker
5. Update status
6. Store results

## 🛠 Development

### Prerequisites

- Go 1.23+
- Docker & Docker Compose
- Make (optional)

### Environment Setup

```bash
DATABASE_URL=postgres://admin:admin@postgres-db:5432/taskmaster?sslmode=disable
KAFKA_BROKER=kafka:9092
REDIS_ADDR=redis:6379
JWT_SECRET=supersecretkey
```

### API Examples

```bash
# Authentication
POST /api/login
{
    "username": "admin",
    "password": "password"
}

# Job Management
POST /api/jobs
{
    "type": "sendEmail",
    "payload": {
        "to": "user@example.com",
        "subject": "Welcome!",
        "template": "welcome_email"
    }
}

GET /api/jobs/:id
GET /api/jobs
```

## 📝 Contributing

1. Fork repository
2. Create feature branch
3. Commit changes
4. Push to branch
5. Open Pull Request

## 📄 License

MIT License
