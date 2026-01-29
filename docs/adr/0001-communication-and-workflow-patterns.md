# ADR 0001: Communication and Workflow Patterns

## Status
Accepted

## Context
ApptRemind is designed as an event-driven set of services. We need production-grade patterns for:
- reliable event publishing
- handling at-least-once delivery
- cross-service workflows without tight coupling
- minimal synchronous dependencies

## Decision
1. **Async by default:** domain state changes are communicated through Kafka events.
2. **Synchronous calls are the exception:** use **gRPC** for internal "must be sync" reads/commands. Public APIs remain **HTTP/REST** behind the gateway.
3. **Outbox pattern per service** for reliable event publishing.
4. **Inbox pattern per consumer** to ensure idempotent processing.
5. **Saga pattern** for cross-service workflows:
   - prefer choreography (event-driven) first
   - introduce an orchestrator only for complex, multi-step workflows needing centralized state/compensation

## Consequences
- Services remain loosely coupled and can scale independently.
- "Exactly once end-to-end" is not assumed; instead we build idempotency and replay safety.
- Some workflows will be eventually consistent; the gateway/UI must handle that.

