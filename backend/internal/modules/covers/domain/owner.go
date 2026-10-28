package domain

// Owner is the kind of record a stored copy belongs to. The covers module
// mirrors the same way for every owner; each owner keeps its mirror state in
// its own table (see the repository descriptors).
type Owner string

const (
	// OwnerBook: books.cover_* (#489).
	OwnerBook Owner = "book"
	// OwnerScreenWorkPoster: screen_works.poster_copy_* for Commons/TMDB posters.
	OwnerScreenWorkPoster Owner = "screen_work_poster"
)

// Valid reports whether o is a known owner.
func (o Owner) Valid() bool { return o == OwnerBook || o == OwnerScreenWorkPoster }
