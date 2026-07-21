# Identity Events

Not named in the original tree sketch, added because `UserCreated`/`ProjectCreated`/`APIKeyCreated` don't fit naturally under provider, routing, or benchmark — tenancy is its own domain, and forcing these three into an unrelated file would be worse than the tree gaining one file it didn't originally list.

| Event | Producer | Consumers | Payload | Ordering | Retry / DLQ |
|---|---|---|---|---|---|
| `UserCreated` | Identity Service | Notification, Analytics | `User` | None | Moderate |
| `ProjectCreated` | Identity Service | Analytics | `Project` | None | Moderate |
| `APIKeyCreated` | Identity Service | Audit (`EventLog`), Notification | `APIKey` metadata — never the raw key, same rule as the API surface | None | Aggressive — security-relevant |
