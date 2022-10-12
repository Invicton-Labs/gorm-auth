package connectors

import (
	"context"
	"database/sql/driver"

	"github.com/go-sql-driver/mysql"
)

// NewMysqlConnector will create a new driver.Connector for a MySQL database
func NewMysqlConnector(getConfigFunc GetMysqlConfigCallback, shouldReconfigureCallback ShouldReconfigureCallback) driver.Connector {
	return &connector{
		shouldReconfigureFunc: shouldReconfigureCallback,
		getConnector: func(ctx context.Context) (driver.Connector, error) {
			cfg, err := getConfigFunc(ctx)
			if err != nil {
				return nil, err
			}
			return mysql.NewConnector(cfg)
		},
	}
}
