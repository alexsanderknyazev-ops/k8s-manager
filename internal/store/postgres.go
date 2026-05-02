package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"k8s-manager/internal/audit"
	"k8s-manager/internal/rbac"
	"k8s-manager/internal/sessionstore"
)

const sessionMaxAgeSec = 24 * 60 * 60 // 24 часа
const schemaVersion = 2

const (
	RoleAdmin  = "admin"
	RoleViewer = "viewer"
)

type User struct {
	Username     string
	PasswordHash string
	Role         string
}

type Permission struct {
	Subject   string `json:"subject"`
	Namespace string `json:"namespace"`
	Resource  string `json:"resource"`
	Verb      string `json:"verb"`
	GrantedBy string `json:"granted_by,omitempty"`
}

// PostgresStore хранит пользователей и роли в PostgreSQL.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore подключается к БД по dsn и выполняет миграции.
func NewPostgresStore(ctx context.Context, dsn string) (*PostgresStore, error) {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	config.ConnConfig.ConnectTimeout = 10 * time.Second
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	s := &PostgresStore{pool: pool}
	if err := s.migrate(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func (s *PostgresStore) migrate(ctx context.Context) error {
	if _, err := s.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`); err != nil {
		return err
	}
	if _, err := s.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS users (
			id             SERIAL PRIMARY KEY,
			username       TEXT NOT NULL UNIQUE,
			password_hash  TEXT NOT NULL,
			role           TEXT NOT NULL DEFAULT 'viewer',
			created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`); err != nil {
		return err
	}
	if _, err := s.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS sessions (
			session_id  TEXT PRIMARY KEY,
			username    TEXT NOT NULL,
			role        TEXT NOT NULL,
			expires_at  TIMESTAMPTZ NOT NULL
		);
	`); err != nil {
		return err
	}
	if _, err := s.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS audit_log (
			id          SERIAL PRIMARY KEY,
			ts          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			method      TEXT NOT NULL,
			path        TEXT NOT NULL,
			status      INT NOT NULL,
			username    TEXT,
			request_id  TEXT
		);
	`); err != nil {
		return err
	}
	if _, err := s.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS user_permissions (
			id          SERIAL PRIMARY KEY,
			subject     TEXT NOT NULL,
			namespace   TEXT NOT NULL DEFAULT '*',
			resource    TEXT NOT NULL DEFAULT '*',
			verb        TEXT NOT NULL DEFAULT 'read',
			granted_by  TEXT NOT NULL DEFAULT '',
			created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE(subject, namespace, resource, verb)
		);
	`); err != nil {
		return err
	}
	if _, err := s.pool.Exec(ctx, `ALTER TABLE user_permissions ADD COLUMN IF NOT EXISTS granted_by TEXT NOT NULL DEFAULT ''`); err != nil {
		return err
	}
	if _, err := s.pool.Exec(ctx, `ALTER TABLE user_permissions ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`); err != nil {
		return err
	}
	if _, err := s.pool.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES ($1) ON CONFLICT(version) DO NOTHING`, schemaVersion); err != nil {
		return err
	}
	return nil
}

// CreateSession создаёт сессию в БД (sessionstore.Store).
func (s *PostgresStore) CreateSession(ctx context.Context, username, role string) (sessionID string, err error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	sessionID = hex.EncodeToString(b)
	expires := time.Now().Add(sessionMaxAgeSec * time.Second)
	_, err = s.pool.Exec(ctx, `INSERT INTO sessions (session_id, username, role, expires_at) VALUES ($1, $2, $3, $4)`,
		sessionID, username, role, expires)
	return sessionID, err
}

// GetSession возвращает пользователя и роль по session_id, если сессия не истекла.
func (s *PostgresStore) GetSession(ctx context.Context, sessionID string) (username, role string, ok bool) {
	if sessionID == "" {
		return "", "", false
	}
	var exp time.Time
	err := s.pool.QueryRow(ctx, `SELECT username, role, expires_at FROM sessions WHERE session_id = $1`, sessionID).
		Scan(&username, &role, &exp)
	if err != nil {
		return "", "", false
	}
	if time.Now().After(exp) {
		_, _ = s.pool.Exec(ctx, `DELETE FROM sessions WHERE session_id = $1`, sessionID)
		return "", "", false
	}
	return username, role, true
}

// DeleteSession удаляет сессию (sessionstore.Store).
func (s *PostgresStore) DeleteSession(ctx context.Context, sessionID string) {
	_, _ = s.pool.Exec(ctx, `DELETE FROM sessions WHERE session_id = $1`, sessionID)
}

// Ensure PostgresStore implements sessionstore.Store and audit.PersistentStore.
var (
	_ sessionstore.Store    = (*PostgresStore)(nil)
	_ audit.PersistentStore = (*PostgresStore)(nil)
	_ rbac.PermissionStore  = (*PostgresStore)(nil)
)

func (s *PostgresStore) HasPermission(ctx context.Context, subject, namespace, resource, verb string) bool {
	var n int
	err := s.pool.QueryRow(ctx, `
		SELECT COUNT(1) FROM user_permissions
		WHERE subject = $1
		  AND namespace IN ($2, '*')
		  AND resource IN ($3, '*')
		  AND verb IN ($4, '*')
	`, subject, namespace, resource, verb).Scan(&n)
	return err == nil && n > 0
}

func (s *PostgresStore) GrantPermission(ctx context.Context, subject, namespace, resource, verb, grantedBy string) error {
	if namespace == "" {
		namespace = "*"
	}
	if resource == "" {
		resource = "*"
	}
	if verb == "" {
		verb = "read"
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO user_permissions(subject, namespace, resource, verb, granted_by)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT(subject, namespace, resource, verb)
		DO UPDATE SET granted_by = EXCLUDED.granted_by, updated_at = NOW()
	`, subject, namespace, resource, verb, grantedBy)
	return err
}

func (s *PostgresStore) SeedOIDCTestPermissions(ctx context.Context, adminEmail, viewerEmail string) error {
	if adminEmail != "" {
		_ = s.GrantPermission(ctx, adminEmail, "*", "*", "*", "seed")
	}
	if viewerEmail != "" {
		_ = s.GrantPermission(ctx, viewerEmail, "*", "*", "read", "seed")
	}
	return nil
}

// SeedTestUsersAndPermissions создаёт тестовых пользователей для dev и назначает granular права.
// Операция идемпотентна: повторный запуск не создаёт дубликаты.
func (s *PostgresStore) SeedTestUsersAndPermissions(ctx context.Context) error {
	type u struct {
		username string
		password string
		role     string
	}
	users := []u{
		{username: "admin", password: "secret", role: RoleAdmin},
		{username: "viewer", password: "viewer", role: RoleViewer},
		{username: "dev", password: "devpass", role: RoleViewer},
		{username: "ops", password: "opspass", role: RoleViewer},
	}
	for _, usr := range users {
		if err := s.CreateUser(ctx, usr.username, usr.password, usr.role); err != nil {
			return err
		}
	}
	// admin в dev-режиме: полный доступ ко всем ресурсам/namespace/verb,
	// иначе при строгом granular RBAC UI будет получать 403 на базовые API.
	_ = s.GrantPermission(ctx, "admin", "*", "*", "*", "seed")
	// viewer: read-only везде
	_ = s.GrantPermission(ctx, "viewer", "*", "*", "read", "seed")
	// dev: разработчик — управление деплойментами в default, чтение pods/services
	_ = s.GrantPermission(ctx, "dev", "default", "deployments", "write", "seed")
	_ = s.GrantPermission(ctx, "dev", "default", "pods", "read", "seed")
	_ = s.GrantPermission(ctx, "dev", "default", "services", "read", "seed")
	// ops: операции с pod, чтение деплойментов
	_ = s.GrantPermission(ctx, "ops", "default", "pods", "write", "seed")
	_ = s.GrantPermission(ctx, "ops", "default", "deployments", "read", "seed")
	return nil
}

func (s *PostgresStore) ListPermissions(ctx context.Context, subject, namespace string) ([]Permission, error) {
	q := `SELECT subject, namespace, resource, verb, granted_by FROM user_permissions WHERE 1=1`
	args := []any{}
	if subject != "" {
		args = append(args, subject)
		q += fmt.Sprintf(" AND subject = $%d", len(args))
	}
	if namespace != "" {
		args = append(args, namespace)
		q += fmt.Sprintf(" AND namespace = $%d", len(args))
	}
	q += " ORDER BY subject, namespace, resource, verb"
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Permission{}
	for rows.Next() {
		var p Permission
		if err := rows.Scan(&p.Subject, &p.Namespace, &p.Resource, &p.Verb, &p.GrantedBy); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *PostgresStore) RevokePermission(ctx context.Context, subject, namespace, resource, verb string) error {
	_, err := s.pool.Exec(ctx, `
		DELETE FROM user_permissions
		WHERE subject = $1 AND namespace = $2 AND resource = $3 AND verb = $4
	`, subject, namespace, resource, verb)
	return err
}

func (s *PostgresStore) DeleteSessionsByUsername(ctx context.Context, username string) error {
	if username == "" {
		return nil
	}
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE username = $1`, username)
	return err
}

// Append добавляет запись в audit_log (audit.PersistentStore).
func (s *PostgresStore) Append(ctx context.Context, method, path string, status int, username, requestID string) {
	_, _ = s.pool.Exec(ctx, `INSERT INTO audit_log (method, path, status, username, request_id) VALUES ($1, $2, $3, $4, $5)`,
		method, path, status, username, requestID)
}

// Get возвращает последние записи аудита из БД (audit.PersistentStore).
func (s *PostgresStore) Get(ctx context.Context, limit int) []audit.Entry {
	if limit <= 0 || limit > 1000 {
		limit = 500
	}
	rows, err := s.pool.Query(ctx,
		`SELECT ts, method, path, status, COALESCE(username,''), COALESCE(request_id,'') FROM audit_log ORDER BY id DESC LIMIT $1`, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []audit.Entry
	for rows.Next() {
		var e audit.Entry
		var username, requestID string
		if err := rows.Scan(&e.Time, &e.Method, &e.Path, &e.Status, &username, &requestID); err != nil {
			continue
		}
		e.Username = username
		e.RequestID = requestID
		out = append(out, e)
	}
	// Новые сверху (как в памяти)
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

// GetUserByUsername возвращает пользователя по логину. role по умолчанию viewer.
func (s *PostgresStore) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	var u User
	err := s.pool.QueryRow(ctx,
		`SELECT username, password_hash, COALESCE(NULLIF(role, ''), 'viewer') FROM users WHERE username = $1`,
		username,
	).Scan(&u.Username, &u.PasswordHash, &u.Role)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// GetUser возвращает password_hash и role для проверки логина (интерфейс auth.UserStore).
func (s *PostgresStore) GetUser(ctx context.Context, username string) (passwordHash, role string, err error) {
	u, err := s.GetUserByUsername(ctx, username)
	if err != nil {
		return "", "", err
	}
	return u.PasswordHash, u.Role, nil
}

// CreateUser создаёт пользователя. Пароль передаётся в открытом виде и хэшируется внутри.
func (s *PostgresStore) CreateUser(ctx context.Context, username, password, role string) error {
	if role == "" {
		role = RoleViewer
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx,
		`INSERT INTO users (username, password_hash, role) VALUES ($1, $2, $3) ON CONFLICT (username) DO NOTHING`,
		username, string(hash), role,
	)
	return err
}

// ListUsers возвращает список пользователей без паролей (username, role).
func (s *PostgresStore) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT username, '' AS password_hash, COALESCE(NULLIF(role, ''), 'viewer') FROM users ORDER BY username`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.Username, &u.PasswordHash, &u.Role); err != nil {
			return nil, err
		}
		list = append(list, u)
	}
	return list, rows.Err()
}

// UpdateRole обновляет роль пользователя.
func (s *PostgresStore) UpdateRole(ctx context.Context, username, role string) error {
	if role != RoleAdmin && role != RoleViewer {
		role = RoleViewer
	}
	_, err := s.pool.Exec(ctx, `UPDATE users SET role = $1 WHERE username = $2`, role, username)
	if err != nil {
		return err
	}
	return s.DeleteSessionsByUsername(ctx, username)
}

// SetPassword устанавливает новый пароль (хэш) для пользователя.
func (s *PostgresStore) SetPassword(ctx context.Context, username, newPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `UPDATE users SET password_hash = $1 WHERE username = $2`, string(hash), username)
	if err != nil {
		return err
	}
	return s.DeleteSessionsByUsername(ctx, username)
}

// DeleteUser удаляет пользователя по логину.
func (s *PostgresStore) DeleteUser(ctx context.Context, username string) error {
	_, _ = s.pool.Exec(ctx, `DELETE FROM sessions WHERE username = $1`, username)
	_, err := s.pool.Exec(ctx, `DELETE FROM users WHERE username = $1`, username)
	return err
}

// SeedAdminIfEmpty создаёт первого админа, если в таблице нет пользователей.
func (s *PostgresStore) SeedAdminIfEmpty(ctx context.Context, username, password string) (created bool, err error) {
	var n int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		return false, err
	}
	if n > 0 {
		return false, nil
	}
	if username == "" || password == "" {
		return false, nil
	}
	err = s.CreateUser(ctx, username, password, RoleAdmin)
	if err != nil {
		return false, err
	}
	slog.Info("postgres: seeded first admin user", "username", username)
	return true, nil
}

// SeedDefaultUsersIfEmpty создаёт по одному пользователю на роль (admin и viewer), если таблица пуста.
// adminUser/adminPass — логин/пароль админа; viewerUser/viewerPass — логин/пароль зрителя.
func (s *PostgresStore) SeedDefaultUsersIfEmpty(ctx context.Context, adminUser, adminPass, viewerUser, viewerPass string) (created bool, err error) {
	var n int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		return false, err
	}
	if n > 0 {
		return false, nil
	}
	if adminUser == "" {
		adminUser = "admin"
	}
	if adminPass == "" {
		adminPass = "secret"
	}
	if viewerUser == "" {
		viewerUser = "viewer"
	}
	if viewerPass == "" {
		viewerPass = "viewer"
	}
	if err := s.CreateUser(ctx, adminUser, adminPass, RoleAdmin); err != nil {
		return false, err
	}
	slog.Info("postgres: seeded default admin", "username", adminUser)
	if err := s.CreateUser(ctx, viewerUser, viewerPass, RoleViewer); err != nil {
		return false, err
	}
	slog.Info("postgres: seeded default viewer", "username", viewerUser)
	return true, nil
}

// Close закрывает пул соединений.
func (s *PostgresStore) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}

// UserManager — интерфейс для API управления пользователями (только при использовании Postgres).
type UserManager interface {
	GetUser(ctx context.Context, username string) (passwordHash, role string, err error)
	ListUsers(ctx context.Context) ([]User, error)
	UpdateRole(ctx context.Context, username, role string) error
	SetPassword(ctx context.Context, username, newPassword string) error
	CreateUser(ctx context.Context, username, password, role string) error
	DeleteUser(ctx context.Context, username string) error
}

type PermissionManager interface {
	ListPermissions(ctx context.Context, subject, namespace string) ([]Permission, error)
	GrantPermission(ctx context.Context, subject, namespace, resource, verb, grantedBy string) error
	RevokePermission(ctx context.Context, subject, namespace, resource, verb string) error
}
