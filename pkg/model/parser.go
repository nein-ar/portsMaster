package model

import "context"

// Parser defines the interface for parsing a single port's metadata.
type Parser interface {
	Parse(category, name string) (*Port, error)
	Type() string
}

// Scanner defines the interface for discovering categories and ports in a tree.
type Scanner interface {
	Scan(ctx context.Context) ([]*Category, []*Port, error)
	Type() string
}

// Source combines scanning and enrichment logic.
type Source interface {
	Fetch(ctx context.Context) (*Database, error)
}
