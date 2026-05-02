package sessionstore

import "context"

// Store — хранилище сессий (память или Postgres). Для продакшена с replicas > 1 нужна общая хранилище (Postgres).
type Store interface {
	CreateSession(ctx context.Context, username, role string) (sessionID string, err error)
	GetSession(ctx context.Context, sessionID string) (username, role string, ok bool)
	DeleteSession(ctx context.Context, sessionID string)
}
