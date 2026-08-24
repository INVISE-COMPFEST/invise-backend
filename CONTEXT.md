# Context: Invise Backend

## Domain Glossary

- **User**: A registered account with an email address, hashed password, and verification status. Identified by a ULID primary key.

- **OTP (One-Time Password)**: A 6-digit numeric code sent via email during registration. Stored in Valkey with a configurable TTL (default 5 minutes). Used to verify the user's email address.

- **Access Token**: A JWT bearer token issued on successful login. Contains `user_id`, `email`, and `role` claims. Signed with HS256. Configurable expiry (default 60 minutes).

- **Verified**: A boolean flag on User. Unverified users cannot log in. Set to `true` after successful OTP verification.

- **Deadstock**: Inventory items that have not sold within a defined period. The system analyzes stock levels, sales data, and unit costs to diagnose deadstock status and project future trends.

- **Stock**: A group of inventory items sharing a common identifier. Each stock contains multiple items tracked by SKU.

- **Projection**: A forecast of a stock's future performance based on historical sales data, producing a projection percentage, decision recommendation, and time-series points.
