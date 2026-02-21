# HorneroDB E-commerce Booking Demo

## Architecture Analysis

HorneroDB's backend is built with Go (using Gin) and uses PostgreSQL (with GORM). The frontend is built in React.
It acts as a low-code database with granular permissions (table, row, column level).
Current findings regarding the user request:

1. **Table/Column Visibility Toggles**: The `metadata.Permission` model supports column-level restrictions (`ColumnID` inside the permission table). However, returning filtered data by stripping unallowed columns based on public roles needs to be ensured/implemented in the `GET /api/v1/workspaces/:ws/data/:slug` endpoint. The UI also needs the "3 puntitos" (three dots) toggle to easily set these column/table permissions for the anonymous/public role.
2. **API Rate Limiting**: Limit API calls per minute per user. This is currently not present in the middleware. Needs a rate limiter middleware configurable globally or per workspace in HorneroDB.
3. **Domain Restriction (CORS)**: Access must be restricted to `turnos.pelucasistemas.com`. The `cors.New` setup in `main.go` currently hardcodes localhost origins. This needs to be made configurable per workspace or as a global environment/database setting.

## Demo Site Structure

### HorneroDB Configuration Needs

**Schemas required:**
1. `Turnos` (Appointments):
   - `from` (DateTime) - PUBLIC visibility
   - `to` (DateTime) - PUBLIC visibility
   - `client_name` (Text) - HIDDEN from public
   - `client_email` (Text) - HIDDEN from public
   - `client_phone` (Text) - HIDDEN from public
2. `Precios` (Prices):
   - `service` (Text) - PUBLIC
   - `price` (Number) - PUBLIC (read-only for all, updateable by specific roles)
3. `Usuarios` (Users): Managed via HorneroDB native auth/roles (Admin, Employee, etc.).

### 1. Public Booking Site (`turnos.pelucasistemas.com`)
* **Role**: Anonymous/Public
* **Displays**: The `Turnos` table schedule (only `from` and `to` visible) to allow selection of available slots. It can also read the `Precios` table if needed.
* **Security**: Enforced by HorneroDB's backend. Rate limited per user/IP. Restricted via CORS to its domain.

### 2. Admin Site
* **Role**: Authenticated (Admin/Employee) via HorneroDB SSO (PocketID/OIDC).
* **Displays**: Full `Turnos` table with PII info.
* **Permissions**: Can manage `Turnos`. Can modify `Precios` if the role permits. Can manage users (add/remove).

## Progress

| Task | Status |
| :--- | :--- |
| Analyze HorneroDB Architecture | Done |
| Plan Demo Structure & API changes | Done |
| Implement Security Toggles (Backend & UI) | Pending |
| Implement Rate Limiting | Pending |
| Implement Configurable CORS | Pending |
| Build Booking Demo Site (Frontend) | Pending |
| Build Admin Demo Site (Frontend) | Pending |
