package shared

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

type DBConfig struct {
	Username string
	Password string
	Host     string
	Port     string
}

type DBHandler struct {
	db *sql.DB
}

func NewDBConfig(username, password, host, port string) *DBConfig {
	return &DBConfig{
		Username: username,
		Password: password,
		Host:     host,
		Port:     port,
	}
}

func NewDBHandler(config *DBConfig) (*DBHandler, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/",
		config.Username,
		config.Password,
		config.Host,
		config.Port,
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %v", err)
	}

	return &DBHandler{db: db}, nil
}

func (h *DBHandler) CreateDatabase(dbName string) error {
	query := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", dbName)
	_, err := h.db.Exec(query)
	return err
}

func (h *DBHandler) DropDatabase(dbName string) error {
	query := fmt.Sprintf("DROP DATABASE IF EXISTS %s", dbName)
	_, err := h.db.Exec(query)
	return err
}

func (h *DBHandler) UseDatabase(dbName string) error {
	query := fmt.Sprintf("USE %s", dbName)
	_, err := h.db.Exec(query)
	return err
}

func (h *DBHandler) CreateTable(req *CreateTableRequest) error {
	columns := make([]string, len(req.Columns))
	for i, col := range req.Columns {
		nullable := "NOT NULL"
		if col.Nullable {
			nullable = "NULL"
		}
		defaultVal := ""
		if col.Default != "" {
			defaultVal = fmt.Sprintf("DEFAULT %s", col.Default)
		}
		columns[i] = fmt.Sprintf("%s %s %s %s", col.Name, col.Type, nullable, defaultVal)
	}

	query := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s.%s (%s)",
		req.DBName, req.TableName, strings.Join(columns, ", "))
	_, err := h.db.Exec(query)
	return err
}

func (h *DBHandler) DropTable(dbName, tableName string) error {
	query := fmt.Sprintf("DROP TABLE IF EXISTS %s.%s", dbName, tableName)
	_, err := h.db.Exec(query)
	return err
}

func (h *DBHandler) ExecuteQuery(query string) (int64, error) {
	result, err := h.db.Exec(query)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (h *DBHandler) QueryRows(query string) (*sql.Rows, error) {
	return h.db.Query(query)
}

func (h *DBHandler) Close() error {
	return h.db.Close()
}

func (h *DBHandler) ReplicateData(req *ReplicationRequest) error {
	switch req.Operation {
	case "INSERT":
		return h.handleInsert(req)
	case "UPDATE":
		return h.handleUpdate(req)
	case "DELETE":
		return h.handleDelete(req)
	default:
		return fmt.Errorf("unsupported operation: %s", req.Operation)
	}
}

func (h *DBHandler) handleInsert(req *ReplicationRequest) error {
	query := fmt.Sprintf("INSERT INTO %s.%s VALUES (%s)", req.DBName, req.TableName, string(req.Data))
	_, err := h.db.Exec(query)
	return err
}

func (h *DBHandler) handleUpdate(req *ReplicationRequest) error {
	query := fmt.Sprintf("UPDATE %s.%s SET %s", req.DBName, req.TableName, string(req.Data))
	_, err := h.db.Exec(query)
	return err
}

func (h *DBHandler) handleDelete(req *ReplicationRequest) error {
	query := fmt.Sprintf("DELETE FROM %s.%s WHERE %s", req.DBName, req.TableName, string(req.Data))
	_, err := h.db.Exec(query)
	return err
}
