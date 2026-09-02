package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
}

func NewPool(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	connString := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName,
	)
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, fmt.Errorf("postgres: new pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres: ping: %w", err)
	}
	return pool, nil
}

// NewPoolFromAddr connects using an addr of the form "host:port", which is
// useful when connecting from outside the Docker network.
func NewPoolFromAddr(ctx context.Context, addr, user, password, dbname string) (*pgxpool.Pool, error) {
	host, port, err := splitHostPort(addr)
	if err != nil {
		return nil, err
	}
	return NewPool(ctx, Config{Host: host, Port: port, User: user, Password: password, DBName: dbname})
}

func splitHostPort(addr string) (string, int, error) {
	i := strings.LastIndex(addr, ":")
	if i < 0 {
		return "", 0, fmt.Errorf("postgres: addr %q missing port", addr)
	}
	var port int
	if _, err := fmt.Sscanf(addr[i+1:], "%d", &port); err != nil {
		return "", 0, fmt.Errorf("postgres: addr %q: %w", addr, err)
	}
	return addr[:i], port, nil
}
