// Package internal holds domain types shared across feature packages
// (auth, posts, comments, reactions) to avoid import cycles.
package internal

type Category struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}
