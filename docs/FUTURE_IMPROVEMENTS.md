# Future Improvements & Technical Debt

This document tracks improvements that should be implemented when scaling HorneroDB beyond single-server deployments.

---

## 🔴 HIGH PRIORITY

### 1. Distributed Rate Limiting with Redis

**Problem:** Current rate limiting uses in-memory maps (`map[string]*visitor`), which means:
- Each server instance has its own isolated rate limiter
- In a multi-server deployment, users can bypass rate limits by hitting different servers
- No shared state between instances

**Current Code:** `internal/middleware/ratelimit.go`
```go
var limiter = &RateLimiter{
    visitors: make(map[string]*visitor),  // Local to each server instance
}
```

**Solution:** Replace with Redis-based rate limiting

**Implementation Example:**
```go
// New: Redis-based rate limiting
import "github.com/go-redis/redis/v8"

func (rl *RateLimiter) AllowRequest(ctx context.Context, key string, limit int, window time.Duration) bool {
    // Use Redis INCR with expiration
    pipe := redisClient.Pipeline()
    incr := pipe.Incr(ctx, fmt.Sprintf("rate_limit:%s", key))
    pipe.Expire(ctx, fmt.Sprintf("rate_limit:%s", key), window)
    _, err := pipe.Exec(ctx)
    
    if err != nil {
        // Fallback to allow on Redis failure (fail open) or block (fail closed)
        return false
    }
    
    return incr.Val() <= int64(limit)
}
```

**Docker Compose Addition:**
```yaml
services:
  redis:
    image: redis:7-alpine
    volumes:
      - redis_data:/data
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 5

volumes:
  redis_data:
```

**Configuration:**
```bash
# .env
REDIS_URL=redis://redis:6379
RATE_LIMITER=redis  # Options: memory (default), redis
```

**Migration Path:**
1. Add Redis service to docker-compose.yml
2. Create `internal/ratelimit/redis.go` with Redis implementation
3. Add configuration option to choose between memory/redis
4. Default to memory for single-server, require Redis for multi-server

**Benefits:**
- ✅ Shared rate limit state across all server instances
- ✅ Persistence across server restarts
- ✅ Can implement more sophisticated algorithms (sliding window, etc.)

**Effort:** Medium (2-4 hours)
**Dependencies:** Redis server

---

## 🟡 MEDIUM PRIORITY

### 2. Move ResolveUserRole to Auth Service

**Problem:** Currently in `internal/middleware/auth.go` as a helper function.

**TODO:** When the auth service grows, move `ResolveUserRole()` to:
```
internal/services/auth/role.go
```

This will make it a proper service method with:
- Caching of role lookups
- Better testability
- Clearer separation of concerns

**Current Location:** `internal/middleware/auth.go` (marked with TODO comment)
**Target Location:** `internal/services/auth/role.go`

---

## 🟢 LOW PRIORITY

### 3. API Key Rotation Automation

**Current:** Manual rotation via API endpoint
**Future:** Automated rotation with:
- Expiration warnings
- Grace period with both keys active
- Automatic cleanup of expired keys

---

## 📊 Infrastructure Scaling Checklist

When moving from single-server to multi-server deployment:

- [ ] **Redis for Rate Limiting** (see item #1 above)
- [ ] **Redis for Session Storage** (if using server-side sessions)
- [ ] **PostgreSQL Connection Pooling** (PgBouncer)
- [ ] **Load Balancer Health Checks** (verify all instances healthy)
- [ ] **Centralized Logging** (ELK stack or similar)
- [ ] **Distributed Tracing** (Jaeger/Zipkin for request tracing)
- [ ] **Webhook Delivery Guarantees** (already implemented with outbox pattern ✅)

---

## 📝 Notes

- All "FUTURE" comments in code should reference this document
- When implementing, move items from this doc to CHANGELOG.md
- Priority order: Security > Performance > Features

---

Last updated: 2024
