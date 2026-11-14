package repository

// pageSize is the number of records returned per page by paginated queries.
const pageSize = 10

// offsetForPage returns the SQL OFFSET for a 1-based page.
// The HTTP layer validates that page is within its allowed range before the
// value reaches a repository.
func offsetForPage(page int) int {
	return (page - 1) * pageSize
}
