# Comprehensive Technical Gap Analysis: `kgb-ai` & `go-utilities`

**Target Repositories:**
- `c:\Bignya\MyProjects\kgb\kgb-ai`
- `c:\Bignya\MyProjects\kgb\go-utilities`

---

## Executive Summary

A comprehensive architectural and code-level gap analysis was conducted across `kgb-ai` and `go-utilities`, evaluating LLM/Embedding pipelines, queuing & worker semantics, resiliency, telemetry, safety controls, and inter-service contracts. 

Several **Critical** architectural deficiencies and security vulnerabilities were identified:
1. **Critical SQL Injection** in RAG dynamic metadata filtering (`kgb-ai-rag`).
2. **Critical Cross-Tenant Isolation Bypass** in MCP tool endpoints accepting unauthenticated caller-supplied `tenant_id`.
3. **Queue Architecture Flaw**: `kgb-ai-workers` uses Redis Pub/Sub broadcast rather than consumer groups/streams, causing all worker replicas to process identical jobs concurrently.
4. **Dropped Dead-Letter Events**: Outbox relay silently ignores events exceeding max retries with no DLQ or alerting.
5. **Runtime Crash in Python Summarizer Agent**: Calling `loop.run_until_complete` inside active asyncio event loop.
6. **HTTP Client Connection Pool Starvation & Blast Radius**: Bare `http.Transport` defaults to 2 conns/host and shared single Hystrix command.

---

## Severity-Categorized Gap Matrix

| ID | Category | Severity | Component | Finding Summary |
| :--- | :--- | :---: | :--- | :--- |
| **SEC-01** | Safety / Security | **CRITICAL** | `kgb-ai-rag` | SQL Injection via unescaped metadata filter keys |
| **SEC-02** | Multi-Tenancy | **CRITICAL** | `kgb-ai-rag` | Unauthenticated, LLM-controlled `tenant_id` in MCP tools |
| **ARC-01** | Job Queuing | **CRITICAL** | `kgb-ai-workers` / `pubsub` | Redis Pub/Sub fanout used as queue; duplicate executions & data races |
| **REL-01** | PubSub / Outbox | **CRITICAL** | `go-utilities/outbox` | **[RESOLVED]** Added DLQStream option, OnDeadLetter callback, and retry dead-letter tombstoning |
| **BUG-01** | Agent Execution | **CRITICAL** | `kgb-ai-agents` | Summarizer agent crashes with `RuntimeError: This event loop is already running` |
| **NET-01** | HTTP Resiliency | **HIGH** | `go-utilities/httpclient` | **[RESOLVED]** Cloned DefaultTransport, pooled conns (100 total / 50 per host), host-scoped Hystrix commands |
| **CTX-01** | Observability | **HIGH** | `go-utilities/httpclient` | **[RESOLVED]** Added `DoRequestWithContext` respecting context cancellation and distributed tracing headers |
| **DB-01** | Multi-Tenancy / DB | **HIGH** | `go-utilities/database` | **[RESOLVED]** Enforced explicit reset of tenant and bypass session variables on un-contexted acquire; added `WithTenantTxContext` |
| **LLM-01** | LLM Integration | **HIGH** | `kgb-ai-agents` / `kgb-ai-rag` | Fallback models and providers completely absent; no rate-limit (429) backoff |
| **SAF-01** | Safety / Prompt Inj | **HIGH** | `kgb-ai-agents` | Guardrails inspect empty action objects; prompt injection defense absent |
| **STB-01** | Architecture | **HIGH** | `kgb-ai-agent-api` | Asynchronous task execution (`ScheduleTask`) is a non-functional stub |
| **STB-02** | Worker Logic | **HIGH** | `kgb-ai-workers` | Delayed retry is a no-op; worker status & results never persisted |
| **CTR-01** | Contracts | **HIGH** | Inter-Service | Incompatible schemas between `Event` (messaging/CRM) and `Job` (AI workers) |
| **DB-02** | DB Resiliency | **MEDIUM** | `go-utilities/database` | **[RESOLVED]** Configured MaxConnIdleTime, MaxConnLifetime, and MinConns on pgxpool |
| **TEL-01** | Telemetry | **MEDIUM** | `go-utilities/otel` | **[RESOLVED]** Added nil provider defensive guards to CustomSpanMiddleware and MetricsMiddleware |
| **AUTH-01** | Authentication | **MEDIUM** | `go-utilities/jwt` | **[RESOLVED]** Added automatic cache invalidation and single-flight refresh on unknown kid in JWT verification |
| **WS-01** | WebSockets | **MEDIUM** | `go-utilities/websocket` | In-memory only hub without distributed Redis pubsub backplane |
| **CTR-02** | Serialization | **MEDIUM** | `kgb-ai-agent-api` | Lossy `fmt.Sprintf("%v", v)` protobuf conversion corrupts nested structures |
| **COST-01** | Monitoring | **LOW** | `kgb-ai-agents` | Hardcoded dummy `$0.01` cost increment per task step |

---

## 1. Deep Dive: `kgb-ai`

### 1.1 Pipeline Architecture & Agent Execution
- **Non-Functional Async Task Scheduling:**
  - **Location:** `kgb-ai/ai-services/kgb-ai-agent-api/internal/service/AgentService.go#L79-L82`
  - **Issue:** `ScheduleTask` is a stub returning a random UUID (`uuid.New().String()`). It does not enqueue tasks to Redis, Kafka, or a database.
  - **Downstream Consequence:** `AgentHandler.ExecuteTaskAsync` returns `202 Accepted` with a `task_id` and status `"queued"`, but no task is ever queued or executed. Furthermore, the URL returned (`/api/v1/agent/tasks/{id}`) does not exist anywhere in the router.
- **In-Memory Session Volatility:**
  - **Location:** `kgb-ai/ai-services/kgb-ai-agents/src/server/grpc_server.py#L38-L56`
  - **Issue:** `_session_service = InMemorySessionService()` stores all agent session state in volatile memory.
  - **Impact:** Any Kubernetes pod restart or rolling deploy wipes active task state and conversation history.
- **Async Event Loop Collision Crash:**
  - **Location:** `kgb-ai/ai-services/kgb-ai-agents/src/adk_agents/summarizer_agent.py#L50-L58`
  - **Issue:** `transcribe_media` attempts to drive `agent.transcribe_audio_url` by invoking `loop.run_until_complete(...)`.
  - **Impact:** Since Google ADK and gRPC run within an active asyncio event loop, this call triggers `RuntimeError: This event loop is already running`, causing the agent to crash. Furthermore, `MockTranscriptionAgent` is hardcoded.

### 1.2 LLM / Embedding Provider Integrations
- **Orphaned Providers & Fallback Model:**
  - **Location:** `kgb-ai/ai-services/kgb-ai-agents/src/config.py#L18-L24`
  - **Issue:** Settings define `fallback_model = "gpt-4o-mini"`, `anthropic_api_key`, and `openai_api_key`. However, neither Anthropic nor OpenAI SDKs are implemented. The fallback model is logged in `main.py#L59` and never referenced again.
  - **Impact:** If Google GenAI rate limits or experiences downtime, the system fails hard with no automatic fallback.
- **Provider Lock-in & Brittle Embeddings:**
  - **Location:** `kgb-ai/ai-services/kgb-ai-rag/internal/kgb-ai-rag/service/Embedding/Service.go#L82-L91`
  - **Issue:** Directly coupled to `go-openai`. It does not support Ollama, Gemini Embeddings, or local models.
  - **Impact:** No retry or exponential backoff on HTTP 429 / 5xx errors; a single failed text in `EmbedBatch` aborts the entire batch.

### 1.3 Job Queuing & Worker Processing
- **Redis Pub/Sub Fanout Anti-Pattern:**
  - **Location:** `kgb-ai/ai-services/kgb-ai-workers/internal/kgb-ai-workers/worker/Worker.go#L60`
  - **Issue:** Workers subscribe via `w.pubsub.Subscribe(ctx, channel, ...)` which maps to Redis `SUBSCRIBE` (broadcast), not Redis Streams consumer groups (`XREADGROUP`) or worker queues.
  - **Impact:** If 3 worker replicas run in production, **all 3 replicas receive and execute the exact same job**, causing race conditions and tripling downstream API/LLM costs.
- **Unimplemented Retry & Delayed Queue:**
  - **Location:** `kgb-ai/ai-services/kgb-ai-workers/internal/kgb-ai-workers/worker/Worker.go#L163-L171`
  - **Issue:** `requeueJobWithDelay` contains only a `// TODO` comment and returns `nil`. When retrying, the job is not re-queued and is permanently dropped.
- **Ephemeral State Tracking:**
  - **Location:** `kgb-ai/ai-services/kgb-ai-workers/internal/kgb-ai-workers/worker/Worker.go#L149-L157`
  - **Issue:** When a job completes, `job.Status = JobStatusCompleted` and `job.Result` are assigned to an in-memory Go struct, but never saved to PostgreSQL or Redis.
- **Unimplemented Handlers:**
  - **Location:** `kgb-ai/ai-services/kgb-ai-workers/internal/kgb-ai-workers/handler/AnalysisHandler.go#L125-L146`
  - **Issue:** `HandleProposalAnalysis` and `HandleLeadAnalysis` are empty stubs returning `JobResult{Success: true}`.

### 1.4 Prompt Injection & Safety Controls
- **CRITICAL SQL Injection Vulnerability:**
  - **Location:** `kgb-ai/ai-services/kgb-ai-rag/internal/kgb-ai-rag/service/VectorStore/Search.go#L208-L212`
  - **Code:**
    ```go
    for key, value := range filters {
        whereConditions += fmt.Sprintf(" AND metadata->>'%s' = $%d", key, argIdx)
        args = append(args, value)
        argIdx++
    }
    ```
  - **Impact:** Metadata filter keys are interpolated directly into SQL without sanitization or identifier validation, allowing SQL injection via crafted filter keys.
- **CRITICAL Multi-Tenant Security Bypass in MCP Server:**
  - **Location:** `kgb-ai/ai-services/kgb-ai-rag/internal/kgb-ai-rag/handler/mcp_server.go#L40-L70`, `#L110-L136`
  - **Issue:** `rag_semantic_search` and `rag_generate_answer` accept `tenant_id` as an unvalidated parameter supplied by the LLM tool invocation over SSE without verifying caller credentials.
  - **Impact:** An attacker can manipulate an LLM prompt to search across other tenants' vector stores.
- **Bypassed Guardrails:**
  - **Location:** `kgb-ai/ai-services/kgb-ai-agents/src/adk_agents/callbacks/policy_guard.py#L101-L112`
  - **Issue:** `action_info` passed to `_guardrails.check_action` contains only `name`, `tenant_id`, and `iteration`. The actual user prompt and tool arguments are never inspected. SQL injection regexes in `guardrails.py` only scan the agent's name.
- **Dead Safety Code & Missing Approvals:**
  - **Location:** `kgb-ai/ai-services/kgb-ai-agents/src/safety/policy.py#L14-L135`
  - **Issue:** `ExecutionPolicy` and `ApprovalWorkflow` are orphaned. Agents like `crm_assistant` are instructed to "Confirm destructive operations with the user," but there is no approval callback connected.
- **Prompt Injection Vulnerability in RAG:**
  - **Location:** `kgb-ai/ai-services/kgb-ai-rag/internal/kgb-ai-rag/service/RAG/Service.go#L200-L208`
  - **Issue:** Context and question are raw-string formatted into the prompt without structural delimiters (e.g. `<context>`) or jailbreak filtering.

---

## 2. Deep Dive: `go-utilities`

### 2.1 HTTP & Database Client Resiliency
- **Connection Pool Starvation:**
  - **Location:** `go-utilities/httpclient/HttpClient.go#L408-L432`
  - **Issue:** `createCustomTransport` instantiates a bare `&http.Transport{TLSClientConfig: tlsConfig}` without setting `MaxIdleConns`, `MaxIdleConnsPerHost`, `IdleConnTimeout`, or keep-alive timeouts.
  - **Impact:** Reverts to Go default `MaxIdleConnsPerHost = 2`. Under moderate load, connections cannot be reused, causing TCP socket exhaustion (TIME_WAIT spikes) and severe latency degradation.
- **Global Hystrix Circuit Breaker Blast Radius:**
  - **Location:** `go-utilities/httpclient/HttpClient.go#L95`, `#L161-L170`
  - **Issue:** `CircuitBreakerCommand` defaults to `"http-client"`.
  - **Impact:** All outgoing HTTP requests across disparate services share a single Hystrix circuit breaker command. If one external endpoint (e.g., an external webhook or AI API) starts timing out, the circuit breaker opens globally, blocking all other outbound HTTP calls across the entire service.
- **Missing Context & Tracing Propagation in HTTP Client:**
  - **Location:** `go-utilities/httpclient/HttpClient.go#L35`, `#L262-L283`
  - **Issue:** `Client` interface and `DoRequest` do not accept `ctx context.Context`. They construct `http.NewRequest(method, finalURL, body)` using background context.
  - **Impact:** Upstream HTTP cancellations (e.g. client disconnects) do not cancel outbound calls, wasting resources. Furthermore, `X-Trace-ID` is never propagated from the incoming request context.
- **PgxPool Idle / Lifetime Settings Ignored:**
  - **Location:** `go-utilities/database/Connection.go#L151-L181`
  - **Issue:** For `PostgresDriver`, only `MaxConns` is set. `PoolConfig.ConnMaxIdleTime` and `PoolConfig.ConnMaxLifetime` are only set on `sql.DB` (line 190) and completely omitted from `pgxpool.Config`.
- **Tenant Context Leakage Risk:**
  - **Location:** `go-utilities/database/tenant.go#L227-L252`
  - **Issue:** In `ConfigureTenantPool`, if `AfterRelease` fails or times out, the error is ignored (`_, _ = conn.Exec(...)`). If the connection is re-acquired by a request lacking a tenant context, the connection retains the previous tenant's setting or `bypass_rls=true`. Additionally, issuing `SELECT set_config` on every acquire and release introduces 2 extra roundtrips per query.
- **Context Discard in `WithTenantTx`:**
  - **Location:** `go-utilities/database/tenant.go#L152-L157`
  - **Issue:** `ctxWithTx` is created but discarded (`_ = ctxWithTx`), meaning callback functions expecting tx in context (`TxFromContext(ctx)`) receive nil.

### 2.2 Outbox & PubSub Guarantees
- **Silent Dropping of Poison-Pill Outbox Events:**
  - **Location:** `go-utilities/outbox/relay.go#L221`
  - **Code:** `WHERE processed_at IS NULL AND retry_count < $2`
  - **Issue:** When an event fails publication `MaxRetries` times, it is excluded from future queries. There is no Dead Letter Queue (DLQ) topic, no dead-letter table, and no alert emitted. The event is silently abandoned.
- **Missing Redis Stream Consumer Implementation:**
  - **Location:** `go-utilities/pubsub/Stream.go#L47-L58`, `Redis.go#L68-L81`
  - **Issue:** `StreamPublisher` provides `PublishStream` (XADD), but `pubsub` provides **zero consumer abstractions** for Redis Streams (`XREADGROUP`, `XACK`, `XAUTOCLAIM`). Services relying on `go-utilities/pubsub` are forced to either use non-durable `rdb.Subscribe` (Pub/Sub) or write raw Redis client logic.
- **Goroutine Leak in Redis Subscribe:**
  - **Location:** `go-utilities/pubsub/Redis.go#L68-L80`
  - **Issue:** `Subscribe` spawns a goroutine listening on `ch := sub.Channel()`. The subscription `sub` is never stored or closed on `RedisPubSub.Close()`.

### 2.3 Logger & Telemetry Correctness
- **Nil Pointer Panics in Telemetry Middleware:**
  - **Location:** `go-utilities/otel/middleware/middleware.go#L29-L30`, `#L65-L66`
  - **Issue:** `CustomSpanMiddleware` and `MetricsMiddleware` immediately call `provider.Tracer(...)` and `provider.Meter(...)` without checking `if provider == nil`. When telemetry is disabled via environment (`InitializeTelemetryFromEnv` returns `nil`), enabling these middlewares causes panic on boot/request.
- **Unimplemented File Logger:**
  - **Location:** `go-utilities/logger/adapters/zerolog/zerolog.go#L192-L195`
  - **Issue:** `setupWriters` has `// TODO: Implement file writer with rotation` when `cfg.Output == "file"`.
- **JWKS Cache Thundering Herd / Invalidation Gap:**
  - **Location:** `go-utilities/jwt/Jwt.go#L21-L32`, `#L240-L243`
  - **Issue:** If Keycloak rotates keys, `getJWKSet` returns cached keys for up to 1 hour without checking if the token's `kid` is missing before invalidating the cache. Also creates a new `httpclient.NewHTTPClient` per cache miss.

---

## 3. Inter-Service Contracts & Backward Compatibility

### 3.1 Serialization & Envelope Inconsistency
- **Event vs. Job Contract Mismatch:**
  - `kgb-messaging` publishes domain events using `Event{Action, Resource, Data, Metadata, Version}` (`Event.go#L28-L36`).
  - `kgb-ai-workers` expects incoming messages to match `Job{ID, Type, Payload, Priority, Status}` (`Models.go#L37-L52`).
  - **Consequence:** `Job` unmarshaling fails or results in empty payloads because the data payload is named `data` in `Event` and `payload` in `Job`. Domain events cannot be processed by AI workers without translation.
- **Ad-hoc Completion Event Envelopes:**
  - **Location:** `kgb-ai/ai-services/kgb-ai-workers/internal/kgb-ai-workers/handler/SummaryHandler.go#L153-L163`
  - `SummaryHandler` publishes `{status: "completed", summary_id: ..., entity_id: ...}` directly to `events:{tenant_id}`. It does not use the standard `Event` envelope or outbox pattern.
- **Inconsistent HTTP Response Envelopes:**
  - `go-utilities/server/Response.go`: `Success` returns unwrapped `data`. `Error` returns `{"error": "message"}`.
  - `kgb-ai-rag/common/Models.go`: Returns `{"success": true, "data": ..., "message": ...}` and `{"error": ..., "message": ..., "code": ...}`.
  - `kgb-ai-agent-api`: Returns `{"error": "...", "details": "..."}`.
- **Lossy Protobuf Context Conversion:**
  - **Location:** `kgb-ai/ai-services/kgb-ai-agent-api/internal/service/AgentService.go#L56-L62`
  - `req.Context` (`map[string]interface{}`) is flattened into `map<string, string>` via `fmt.Sprintf("%v", v)`.
  - Nested JSON objects/arrays are serialized into Go string notation (e.g. `map[key:val]`), which cannot be parsed as valid JSON by downstream Python agents.

---

## Prioritized Remediation Roadmap

### Phase 1: Immediate Critical Fixes (P0)
1. **Fix SQL Injection in RAG Search:**
   - In `kgb-ai-rag/.../VectorStore/Search.go`, validate filter keys against an allowlist or sanitize using `pgx.Identifier{key}.Sanitize()`.
2. **Secure MCP Tenant Isolation:**
   - In `kgb-ai-rag/.../handler/mcp_server.go`, remove `tenant_id` from tool parameters. Extract and enforce `tenant_id` strictly from authenticated SSE connection headers/JWT.
3. **Resolve Asyncio Loop Crash:**
   - In `kgb-ai-agents/.../summarizer_agent.py`, make `transcribe_media` an async tool (`async def transcribe_media`) and await `agent.transcribe_audio_url` directly.
4. **Implement Outbox DLQ:**
   - In `go-utilities/outbox/relay.go`, when `retry_count >= MaxRetries`, move the event to an `outbox_dead_letters` table or publish to a dead-letter stream with an error alert.

### Phase 2: High Priority Architectural Upgrades (P1)
1. **Refactor Queuing to Redis Streams / Consumer Groups:**
   - In `go-utilities/pubsub`, implement `StreamConsumer` with `XREADGROUP` and `XACK`.
   - Update `kgb-ai-workers` to consume from Redis Streams with consumer groups so replicas share workload rather than duplicating execution.
2. **Repair HTTP Client Transport & Blast Radius:**
   - Configure `http.Transport` pool defaults (`MaxIdleConns: 100`, `MaxIdleConnsPerHost: 20`, `IdleConnTimeout: 90s`).
   - Allow per-host / per-service Hystrix command names instead of a shared default `"http-client"`.
   - Add `context.Context` to `Client` and `DoRequest`.
3. **Unify Inter-Service Contracts:**
   - Create a shared `go-utilities/contract` package defining canonical `EventEnvelope` and `JobEnvelope` schemas.
   - Use `json.RawMessage` or JSON string for `TaskRequest.context` in `agent.proto`.
4. **Implement Fallback LLM / Retry Handlers:**
   - Add retry with exponential backoff on HTTP 429 for OpenAI/Gemini in `kgb-ai-rag` and `kgb-ai-agents`.
   - Implement the secondary fallback model (`gpt-4o-mini`) in ADK runner when primary Gemini invocation fails.

### Phase 3: Resiliency & Hygiene Improvements (P2)
1. **Fix Tenant Pool Cleanup:** Ensure `BeforeAcquire` explicitly resets `kgb.tenant_id` and `kgb.bypass_rls` if no tenant is set on context, preventing cross-tenant leakage.
2. **Defend against Panics in Telemetry:** Add nil checks for `provider` in `MetricsMiddleware` and `CustomSpanMiddleware`.
3. **Persist Worker Job State:** Write job progress and completed analysis to PostgreSQL in `kgb-ai-workers`.
4. **JWKS Cache Invalidation:** In `go-utilities/jwt`, force a cache refresh if a token's `kid` is not found in the cached JWKS.
