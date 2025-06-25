package gorm

import (
	"context"
	"database/sql"

	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

// Dialector GORM数据库驱动器接口。
type Dialector interface {
	Name() string
	Initialize(*DB) error
	Migrator(db *DB) Migrator
	DataTypeOf(*schema.Field) string
	DefaultValueOf(*schema.Field) clause.Expression
	BindVarTo(writer clause.Writer, stmt *Statement, v interface{})
	QuoteTo(clause.Writer, string)
	Explain(sql string, vars ...interface{}) string
}

// Plugin GORM插件接口。
type Plugin interface {
	Name() string
	Initialize(*DB) error
}

// ParamsFilter 参数过滤器接口。
type ParamsFilter interface {
	ParamsFilter(ctx context.Context, sql string, params ...interface{}) (string, []interface{})
}

// ConnPool 数据库连接池接口。
type ConnPool interface {
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

// SavePointerDialectorInterface 保存指针接口。
type SavePointerDialectorInterface interface {
	SavePoint(tx *DB, name string) error
	RollbackTo(tx *DB, name string) error
}

// TxBeginner 事务开始器接口。
type TxBeginner interface {
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
}

// ConnPoolBeginner 数据库连接池开始器接口。
type ConnPoolBeginner interface {
	BeginTx(ctx context.Context, opts *sql.TxOptions) (ConnPool, error)
}

// TxCommitter 事务提交器接口。
type TxCommitter interface {
	Commit() error
	Rollback() error
}

// Tx sql.Tx接口。
type Tx interface {
	ConnPool
	TxCommitter
	StmtContext(ctx context.Context, stmt *sql.Stmt) *sql.Stmt
}

// Valuer GORM值器接口。
type Valuer interface {
	GormValue(context.Context, *DB) clause.Expr
}

// GetDBConnector SQL数据库连接器接口。
type GetDBConnector interface {
	GetDBConn() (*sql.DB, error)
}

// Rows 行接口。
type Rows interface {
	Columns() ([]string, error)
	ColumnTypes() ([]*sql.ColumnType, error)
	Next() bool
	Scan(dest ...interface{}) error
	Err() error
	Close() error
}

// ErrorTranslator 错误翻译器接口。
type ErrorTranslator interface {
	Translate(err error) error
}
