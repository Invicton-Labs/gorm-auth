package connectors

import (
	"context"
	"database/sql/driver"
	"fmt"
	"sync"

	"github.com/go-sql-driver/mysql"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/stdlib"
)

// A function signature for a callback function that determines whether the connection
// configuration should be reconfigured for the next connection.
type ShouldReconfigureCallback func(ctx context.Context) (reconfigure bool, err error)

// A function signature for a callback function that gets the Postgres connection configuraiton.
type GetPostgresConfigCallback func(ctx context.Context) (config pgx.ConnConfig, opts []stdlib.OptionOpenDB, err error)

// A function signature for a callback function that gets the MySQL connection configuraiton.
type GetMysqlConfigCallback func(ctx context.Context) (*mysql.Config, error)

type connector struct {
	reconfigureLock       sync.Mutex
	connector             driver.Connector
	shouldReconfigureFunc ShouldReconfigureCallback
	getConnector          func(ctx context.Context) (driver.Connector, error)
}

func (c *connector) Driver() driver.Driver {
	return c
}

func (c *connector) Open(name string) (driver.Conn, error) {
	return nil, fmt.Errorf("open is not supported")
}

func (c *connector) prepareConnector(ctx context.Context) error {
	// Ensure that the connector callbacks are thread-safe
	c.reconfigureLock.Lock()
	defer c.reconfigureLock.Unlock()

	// Check whether we should reconfigure the connector
	var reconfigure bool
	// If there's no connector yet, or there's no callback provided
	// for determining whent to reconfigure, then reconfigure.
	if c.connector == nil || c.shouldReconfigureFunc == nil {
		reconfigure = true
	} else {
		// Otherwise, run the callback to determine if we should reconfigure.
		var err error
		reconfigure, err = c.shouldReconfigureFunc(ctx)
		if err != nil {
			return err
		}
	}

	if reconfigure {
		// Create a new connector
		connector, err := c.getConnector(ctx)
		if err != nil {
			return err
		}
		c.connector = connector
	}

	return nil
}

func (c *connector) Connect(ctx context.Context) (driver.Conn, error) {
	if err := c.prepareConnector(ctx); err != nil {
		return nil, err
	}
	return c.connector.Connect(ctx)
}
