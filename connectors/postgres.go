package connectors

import (
	"context"
	"database/sql/driver"

	"github.com/Invicton-Labs/go-stackerr"
	"github.com/jackc/pgx/v4/stdlib"
)

// NewPostgresConnector will create a new driver.Connector for PostgreSQL
func NewPostgresConnector(getConfigFunc GetPostgresConfigCallback, shouldReconfigureCallback ShouldReconfigureCallback) driver.Connector {
	return &connector{
		shouldReconfigureFunc: shouldReconfigureCallback,
		getConnector: func(ctx context.Context) (driver.Connector, stackerr.Error) {
			cfg, opts, err := getConfigFunc(ctx)
			if err != nil {
				return nil, err
			}
			return stdlib.GetConnector(cfg, opts...), nil
		},
	}
}
