package graph

//go:generate go run github.com/99designs/gqlgen

type Resolver struct {
}

type contextKey string

const ContextKeyClientID contextKey = "client_id"
