package gormauth

import (
	"database/sql"
	"database/sql/driver"
	"time"

	gormmysql "gorm.io/driver/mysql"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type DialectorInput struct {
	ShouldReconfigureCallback ShouldReconfigureCallback
	ConnMaxIdleTime           *time.Duration
	ConnMaxLifetime           *time.Duration
	MaxIdleConns              *int
	MaxOpenConns              *int
}

func (di DialectorInput) Clone() DialectorInput {
	return DialectorInput{
		ShouldReconfigureCallback: di.ShouldReconfigureCallback,
		ConnMaxIdleTime:           di.ConnMaxIdleTime,
		ConnMaxLifetime:           di.ConnMaxIdleTime,
		MaxIdleConns:              di.MaxIdleConns,
		MaxOpenConns:              di.MaxOpenConns,
	}
}

type MysqlDialectorInput struct {
	DialectorInput
	GormMysqlConfig        gormmysql.Config
	GetMysqlConfigCallback GetMysqlConfigCallback
}

type PostgresDialectorInput struct {
	DialectorInput
	GormPostgresConfig        gormpostgres.Config
	GetPostgresConfigCallback GetPostgresConfigCallback
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

func NewDialector[InputType dialectorInputType](input InputType) gorm.Dialector {
	switch in := any(input).(type) {
	case MysqlDialectorInput:
		return NewMysqlDialector(in)
	case PostgresDialectorInput:
		return NewPostgresDialector(in)
	}
	panic("unknown input type")
}

func NewMysqlDialector(input MysqlDialectorInput) gorm.Dialector {
	if input.GetMysqlConfigCallback == nil {
		panic("the `input.GetMysqlConfigCallback` field must not be nil")
	}

	connector := newMysqlConnector(input.GetMysqlConfigCallback, input.ShouldReconfigureCallback)

	input.GormMysqlConfig.Conn = getBaseDb(input.DialectorInput, connector)
	input.GormMysqlConfig.DSN = ""
	if input.GormMysqlConfig.DriverName == "" {
		input.GormMysqlConfig.DriverName = "mysql-gormauth"
	}

	return gormmysql.New(input.GormMysqlConfig)
}

func NewPostgresDialector(input PostgresDialectorInput) gorm.Dialector {
	if input.GetPostgresConfigCallback == nil {
		panic("the `input.GetPostgresConfigCallback` field must not be nil")
	}

	connector := newPostgresConnector(input.GetPostgresConfigCallback, input.ShouldReconfigureCallback)

	input.GormPostgresConfig.Conn = getBaseDb(input.DialectorInput, connector)
	input.GormPostgresConfig.DSN = ""
	if input.GormPostgresConfig.DriverName == "" {
		input.GormPostgresConfig.DriverName = "postgres-gormauth"
	}

	return gormpostgres.New(input.GormPostgresConfig)
}
