package handler

import (
	"errors"
	"net/http"
	"strconv"
)

const (
	// defaultPage is used when the request does not carry a page parameter.
	defaultPage = 1

	// minPage is the first page. Pages are 1-based.
	minPage = 1

	// maxPage bounds the page parameter. Repositories currently read 10 rows
	// per page, so this caps a single response at 1000 rows.
	maxPage = 100
)

var errInvalidPage = errors.New("invalid page parameter")

/*
pageFromQuery parses the 1-based page query parameter of r.

An absent or empty page means defaultPage. Any other value must be an integer
within [minPage, maxPage], otherwise an error is returned so the handler can
answer with 400 instead of passing a bad value to the database.
*/
func pageFromQuery(r *http.Request) (int, error) {
	raw := r.URL.Query().Get("page")
	if raw == "" {
		return defaultPage, nil
	}

	page, err := strconv.Atoi(raw)
	if err != nil || page < minPage || page > maxPage {
		return 0, errInvalidPage
	}

	return page, nil
}
