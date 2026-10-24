# Tasks

## 1. Backend

- [x] 1.1 In the auth service `Register`, trim `name` and reject it unless it has 2–255 characters (`utf8.RuneCountInString`) with a new `ErrInvalidName`, before any DB access; store the trimmed name. Map the error to 400 in the handler. Verify: `go build ./...`, `go vet ./...` from `backend/`.
- [x] 1.2 Tests: whitespace-only name → 400/`ErrInvalidName`, no user created; `"  Анна  "` stored as `"Анна"`; 255 Cyrillic characters accepted; 256 characters rejected (not a 500). Verify: `go test ./internal/modules/auth/...`.

## 2. Frontend (apps/web)

- [x] 2.1 Register form schema: trim the name and require 2–255 characters with «Имя должно содержать от 2 до 255 символов»; the request sends the trimmed name. Extend/add a form test (whitespace-only and 256-character names show the message and send nothing).
- [x] 2.2 Verify: `tsc --noEmit` and `next lint` in `apps/web`.
