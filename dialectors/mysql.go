package dialectors

import (
	"github.com/Invicton-Labs/gorm-auth/connectors"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type MysqlDialectorInput struct {
	// Common (not specific to MySQL) input values
	DialectorInput

	// The GORM-specific MySQL configuration values to use
	GormMysqlConfig gormmysql.Config

	// A function that gets the config to use for the next
	// MySQL connection
	GetMysqlConfigCallback connectors.GetMysqlConfigCallback
}

// Returns a new copy of the MysqlDialectorInput struct
func (di MysqlDialectorInput) Clone() MysqlDialectorInput {
	return MysqlDialectorInput{
		DialectorInput:         di.DialectorInput,
		GormMysqlConfig:        di.GormMysqlConfig,
		GetMysqlConfigCallback: di.GetMysqlConfigCallback,
	}
}

func newMysqlDialector(input MysqlDialectorInput) gorm.Dialector {
	if input.GetMysqlConfigCallback == nil {
		panic("the `input.GetMysqlConfigCallback` field must not be nil")
	}

	connector := connectors.NewMysqlConnector(input.GetMysqlConfigCallback, input.ShouldReconfigureCallback)

	input.GormMysqlConfig.Conn = getBaseDb(input.DialectorInput, connector)
	input.GormMysqlConfig.DSN = ""
	if input.GormMysqlConfig.DriverName == "" {
		input.GormMysqlConfig.DriverName = "mysql-gormauth"
	}

	return gormmysql.New(input.GormMysqlConfig)
}
