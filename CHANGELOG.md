# Changelog

## 0.4.4

Documentation only; the provider's behaviour and schema are unchanged from 0.4.3.

- **`bahriya_container`: `registry`, and an init job's `registry`, take the registry's ID, not its handle.**
  Use `bahriya_registry.<name>.id`. The documentation said both took a handle, and its example set
  `registry = bahriya_registry.ghcr.handle`. That example never worked: the provider sends the value unchanged
  and the API looks it up as an ID, so a handle failed with "An unexpected error occurred" (HTTP 500). The API
  now answers a handle with HTTP 400 and a message saying to send the ID.
