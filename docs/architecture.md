# Patchflow Architecture

## The Millisecond Miracle

The core philosophy of Patchflow is separating the hot path (proxy) from the heavy lifting (AI). 

```mermaid
graph TD
    Client -->|{"charge": 10}| Proxy
    Proxy -->|{"charge": 10}| Stripe[Target API]
    Stripe -.->|400 Bad Request| Proxy
    Proxy -->|Halt & Escalate| Agent[AI Agent]
    Agent -->|Generate Fix: charge -> amount| Proxy
    Proxy -->|{"amount": 10}| Stripe
    Stripe -.->|200 OK| Proxy
    Proxy -.->|200 OK| Client
    Agent -->|Async| PREngine[PR Engine]
    PREngine -->|Open PR #482| GitHub
```

### The Data Plane
The Proxy must never block waiting for AI during standard requests. It relies entirely on a high-speed cache for known fixes.

### The Control Plane
The Agent handles rate limits, API schemas, and context window management for the LLM. It is decoupled completely from the customer's live traffic flow.
