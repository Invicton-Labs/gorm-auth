package dialectors

import (
	"database/sql"
	"database/sql/driver"
	"time"

	"github.com/Invicton-Labs/gorm-auth/connectors"
	"gorm.io/gorm"
)

// The common (relevant to all database types) configuration values that
// should be used for new connections.
type DialectorInput struct {
	// A function that determines whether a new configuration should be used
	// for the next connection
	ShouldReconfigureCallback connectors.ShouldReconfigureCallback
	// The maximum duration to allow a connection to remain idle before closing it
	ConnMaxIdleTime *time.Duration
	// The maximum duration to allow a connection to remain open (regardless of
	// whether it is idle) before closing it
	ConnMaxLifetime *time.Duration
	// The maximum number of idle connections that can remain open
	MaxIdleConns *int
	// The maximum number of connections (regardless of whether they are idle)
	// that can be open at any given time.
	MaxOpenConns *int
}

type dialectorInputType interface {
	MysqlDialectorInput | PostgresDialectorInput
}

func getBaseDb(input DialectorInput, connector driver.Connector) *sql.DB {
	baseDb := sql.OpenDB(connector)

	if input.ConnMaxIdleTime != nil {
		baseDb.SetConnMaxIdleTime(*input.ConnMaxIdleTime)
	}
	if input.ConnMaxLifetime != nil {
		baseDb.SetConnMaxLifetime(*input.ConnMaxLifetime)
	}
	if input.MaxIdleConns != nil {
		baseDb.SetMaxIdleConns(*input.MaxIdleConns)
	}
	if input.MaxOpenConns != nil {
		baseDb.SetMaxOpenConns(*input.MaxOpenConns)
	}

	return baseDb
}

// NewDialector creates a new GORM dialector using the given input configurations.
// It automatically selects between a MySQL or Postgres dialector depending on the
// input configuration type.
func NewDialector[InputType dialectorInputType](input InputType) gorm.Dialector {
	switch in := any(input).(type) {
	case MysqlDialectorInput:
		return newMysqlDialector(in)
	case PostgresDialectorInput:
		return newPostgresDialector(in)
	}
	panic("unknown input type")
}
