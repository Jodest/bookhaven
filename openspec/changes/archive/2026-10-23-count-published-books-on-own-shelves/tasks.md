## 1. Backend

- [x] 1.1 `ListByUser` counts published books only (`LEFT JOIN books ... status = 'published'`, `COUNT(b.id)`); `ListByUserVisible` delegates to it.
- [x] 1.2 Extend `TestShelvesBookStatus`: the owner's count skips pending and rejected books.
- [x] 1.3 `go build`, `go vet`, `go test` for the backend.
