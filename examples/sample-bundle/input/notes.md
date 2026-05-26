# Acme Orders Service

The Acme Orders Service accepts order requests, validates inventory, stores order state, and publishes fulfillment events.

## Components

- HTTP API receives order submissions.
- Inventory client checks stock before confirmation.
- Orders database stores order status.
- Event publisher emits fulfillment events.

## Risks

- Inventory failures delay confirmation.
- Duplicate submissions require idempotency keys.
- Event publishing must not happen before order state is committed.
