# Security: Authentication

JWT-based authentication with bcrypt password hashing and account lockout. Non-negotiable — these rules are audited on every PR.

### JWT Session Cookie Flags

JWT session cookies must be set with `HttpOnly=true`, `Secure=true`, and `SameSite=Strict`. This combination is immune to XSS-based exfiltration and is automatically sent by browsers. Sources: CLAUDE.md, ADR-005.

### Password Complexity & Hashing Policy

Passwords are hashed with bcrypt at cost factor **12**. Password complexity: minimum 10 characters, at least one uppercase, one lowercase, one digit, and one special character. Sources: CLAUDE.md, docs/auth.md, ADR-005.

### Account Lockout Policy (Not Bypassable in Tests)

After 5 consecutive failed login attempts, lock the account for 15 minutes. Do not bypass this behavior in tests — use separate accounts per test instead. Sources: CLAUDE.md, docs/auth.md, ADR-005.

### Lock Check Before Password Verification

Check the lock status before verifying the password. This prevents timing attacks that could leak password validity during a lockout. Source: ADR-005.
