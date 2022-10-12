package dialectors

import (
	"github.com/Invicton-Labs/gorm-auth/connectors"
	"gorm.io/gorm"

	gormpostgres "gorm.io/driver/postgres"
)

type PostgresDialectorInput struct {
	// Common (not specific to PostgreSQL) input values
	DialectorInput

	// The GORM-specific PostgreSQL configuration values to use
	GormPostgresConfig gormpostgres.Config

	// A function that gets the config to use for the next
	// PostgreSQL connection
	GetPostgresConfigCallback connectors.GetPostgresConfigCallback
}

// Returns a new copy of the PostgresDialectorInput struct
func (di PostgresDialectorInput) Clone() PostgresDialectorInput {
	return PostgresDialectorInput{
		DialectorInput:            di.DialectorInput,
		GormPostgresConfig:        di.GormPostgresConfig,
		GetPostgresConfigCallback: di.GetPostgresConfigCallback,
	}
}

func newPostgresDialector(input PostgresDialectorInput) gorm.Dialector {
	if input.GetPostgresConfigCallback == nil {
		panic("the `input.GetPostgresConfigCallback` field must not be nil")
	}

	connector := connectors.NewPostgresConnector(input.GetPostgresConfigCallback, input.ShouldReconfigureCallback)

	input.GormPostgresConfig.Conn = getBaseDb(input.DialectorInput, connector)
	input.GormPostgresConfig.DSN = ""
	if input.GormPostgresConfig.DriverName == "" {
		input.GormPostgresConfig.DriverName = "postgres-gormauth"
	}

	return gormpostgres.New(input.GormPostgresConfig)
}
