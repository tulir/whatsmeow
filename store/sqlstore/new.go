package sqlstore

import (
    "context"
    "fmt"

    "github.com/jackc/pgx/v5/pgxpool"
    waLog "go.mau.fi/whatsmeow/util/log"
)

// New creates a new SQL-backed store.Container.
//
// The function is kept for backwards-compatibility with existing user code
// and examples. At the moment only the "postgres"/"pgx" driver is supported
// in this build. Passing any other driver will return an error.
//
// Example:
//   ctx := context.Background()
//   container, err := sqlstore.New(ctx, "postgres", "postgres://user:pass@localhost/db", nil)
//
// The returned Container can then be used to manage WhatsApp devices.
func New(ctx context.Context, driverName, dsn string, log waLog.Logger) (*Container, error) {
    if log == nil {
        log = waLog.Noop
    }

    switch driverName {
    case "postgres", "pgx", "" /* default */:
        cfg, err := pgxpool.ParseConfig(dsn)
        if err != nil {
            return nil, fmt.Errorf("failed to parse Postgres DSN: %w", err)
        }
        pool, err := pgxpool.NewWithConfig(ctx, cfg)
        if err != nil {
            return nil, fmt.Errorf("failed to connect to Postgres: %w", err)
        }
        return NewContainer(pool, "", log), nil
    default:
        return nil, fmt.Errorf("driver %q not supported in this build", driverName)
    }
}
