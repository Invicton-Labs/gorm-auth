package connectors

import (
	"context"
	"database/sql/driver"

	"github.com/Invicton-Labs/go-stackerr"
	"github.com/go-sql-driver/mysql"
)

// NewMysqlConnector will create a new driver.Connector for a MySQL database
func NewMysqlConnector(getConfigFunc GetMysqlConfigCallback, shouldReconfigureCallback ShouldReconfigureCallback) driver.Connector {
	return &connector{
		shouldReconfigureFunc: shouldReconfigureCallback,
		getConnector: func(ctx context.Context) (driver.Connector, stackerr.Error) {
			cfg, err := getConfigFunc(ctx)
			if err != nil {
				return nil, err
			}
			conn, cerr := mysql.NewConnector(cfg)
			return conn, stackerr.Wrap(cerr)
		},
	}
}
