package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

// DBManager handles PostgreSQL and MySQL connections and operations
type DBManager struct{}

func NewDBManager() *DBManager {
	return &DBManager{}
}

// buildDSN creates connection string
func (dm *DBManager) buildDSN(req DBRequest, targetDB string) (string, string) {
	driver := strings.ToLower(req.Driver)
	if driver == "" || driver == "postgres" || driver == "postgresql" || driver == "pgsql" {
		driver = "postgres"
		host := req.Host
		if host == "" {
			host = "localhost"
		}
		port := req.Port
		if port == 0 {
			port = 5432
		}
		user := req.User
		if user == "" {
			user = "postgres"
		}
		ssl := req.SSLMode
		if ssl == "" {
			ssl = "disable"
		}
		db := targetDB
		if db == "" {
			db = req.DBName
			if db == "" {
				db = "postgres"
			}
		}
		dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s connect_timeout=5",
			host, port, user, req.Password, db, ssl)
		return "postgres", dsn
	}

	// MySQL
	host := req.Host
	if host == "" {
		host = "localhost"
	}
	port := req.Port
	if port == 0 {
		port = 3306
	}
	user := req.User
	if user == "" {
		user = "root"
	}
	db := targetDB
	if db == "" {
		db = req.DBName
	}
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?timeout=5s&parseTime=true",
		user, req.Password, host, port, db)
	return "mysql", dsn
}

// openConn opens connection to DB
func (dm *DBManager) openConn(req DBRequest, targetDB string) (*sql.DB, error) {
	driver, dsn := dm.buildDSN(req, targetDB)
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ошибка подключения (%s): %w", driver, err)
	}
	return db, nil
}

// TestConnection tests whether credentials and server are reachable
func (dm *DBManager) TestConnection(req DBRequest) error {
	db, err := dm.openConn(req, "")
	if err != nil {
		return err
	}
	defer db.Close()
	return nil
}

// ListDatabases returns list of databases on the instance
func (dm *DBManager) ListDatabases(req DBRequest) ([]string, error) {
	driver, _ := dm.buildDSN(req, "")
	var defaultDB string
	if driver == "postgres" {
		defaultDB = "postgres"
	}
	
	db, err := dm.openConn(req, defaultDB)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var query string
	if driver == "postgres" {
		query = "SELECT datname FROM pg_database WHERE datistemplate = false ORDER BY datname;"
	} else {
		query = "SHOW DATABASES;"
	}

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err == nil {
			names = append(names, name)
		}
	}
	return names, nil
}

// CreateDatabase creates a new database
func (dm *DBManager) CreateDatabase(req DBRequest, name string) error {
	if strings.ContainsAny(name, " ;'\"\\-/*") {
		return fmt.Errorf("недопустимое имя базы данных: %s", name)
	}

	driver, _ := dm.buildDSN(req, "")
	var defaultDB string
	if driver == "postgres" {
		defaultDB = "postgres"
	}

	db, err := dm.openConn(req, defaultDB)
	if err != nil {
		return err
	}
	defer db.Close()

	var query string
	if driver == "postgres" {
		query = fmt.Sprintf("CREATE DATABASE \"%s\";", name)
	} else {
		query = fmt.Sprintf("CREATE DATABASE `%s`;", name)
	}

	_, err = db.Exec(query)
	return err
}

// DropDatabase removes a database
func (dm *DBManager) DropDatabase(req DBRequest, name string) error {
	if strings.ContainsAny(name, " ;'\"\\-/*") {
		return fmt.Errorf("недопустимое имя базы данных: %s", name)
	}

	driver, _ := dm.buildDSN(req, "")
	var defaultDB string
	if driver == "postgres" {
		defaultDB = "postgres"
	}

	db, err := dm.openConn(req, defaultDB)
	if err != nil {
		return err
	}
	defer db.Close()

	var query string
	if driver == "postgres" {
		query = fmt.Sprintf("DROP DATABASE IF EXISTS \"%s\";", name)
	} else {
		query = fmt.Sprintf("DROP DATABASE IF EXISTS `%s`;", name)
	}

	_, err = db.Exec(query)
	return err
}

// ListTables returns tables in database
func (dm *DBManager) ListTables(req DBRequest) ([]string, error) {
	db, err := dm.openConn(req, req.DBName)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	driver, _ := dm.buildDSN(req, req.DBName)
	var query string
	if driver == "postgres" {
		query = "SELECT table_name FROM information_schema.tables WHERE table_schema='public' ORDER BY table_name;"
	} else {
		query = "SHOW TABLES;"
	}

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tbl string
		if err := rows.Scan(&tbl); err == nil {
			tables = append(tables, tbl)
		}
	}
	return tables, nil
}

// QueryResult represents tabular SQL results
type QueryResult struct {
	Columns      []string                 `json:"columns"`
	Rows         [][]interface{}          `json:"rows"`
	RowsAffected int64                    `json:"rows_affected"`
	ExecutionMs  int64                    `json:"execution_ms"`
	Message      string                   `json:"message"`
}

// ExecuteQuery runs arbitrary SQL query
func (dm *DBManager) ExecuteQuery(req DBRequest, query string) (*QueryResult, error) {
	db, err := dm.openConn(req, req.DBName)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	start := time.Now()
	trimmed := strings.TrimSpace(strings.ToUpper(query))

	if strings.HasPrefix(trimmed, "SELECT") || strings.HasPrefix(trimmed, "SHOW") || strings.HasPrefix(trimmed, "EXPLAIN") {
		rows, err := db.Query(query)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		cols, err := rows.Columns()
		if err != nil {
			return nil, err
		}

		var resultRows [][]interface{}
		for rows.Next() {
			columnPointers := make([]interface{}, len(cols))
			columnValues := make([]interface{}, len(cols))
			for i := range columnValues {
				columnPointers[i] = &columnValues[i]
			}

			if err := rows.Scan(columnPointers...); err != nil {
				return nil, err
			}

			rowItem := make([]interface{}, len(cols))
			for i, val := range columnValues {
				if b, ok := val.([]byte); ok {
					rowItem[i] = string(b)
				} else {
					rowItem[i] = val
				}
			}
			resultRows = append(resultRows, rowItem)
		}

		return &QueryResult{
			Columns:     cols,
			Rows:        resultRows,
			ExecutionMs: time.Since(start).Milliseconds(),
			Message:     fmt.Sprintf("Успешно: %d строк", len(resultRows)),
		}, nil
	}

	// DDL / DML
	res, err := db.Exec(query)
	if err != nil {
		return nil, err
	}
	affected, _ := res.RowsAffected()
	return &QueryResult{
		RowsAffected: affected,
		ExecutionMs:  time.Since(start).Milliseconds(),
		Message:      fmt.Sprintf("Успешно выполнено. Изменено строк: %d", affected),
	}, nil
}

// ensureKVTable ensures a key-value table exists in target database
func (dm *DBManager) ensureKVTable(db *sql.DB, driver string) error {
	var query string
	if driver == "postgres" {
		query = `CREATE TABLE IF NOT EXISTS node_kv (
			key VARCHAR(255) PRIMARY KEY,
			value TEXT,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`
	} else {
		query = `CREATE TABLE IF NOT EXISTS node_kv (
			` + "`key`" + ` VARCHAR(255) PRIMARY KEY,
			` + "`value`" + ` LONGTEXT,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`
	}
	_, err := db.Exec(query)
	return err
}

// KVRead reads value for specified path/key
func (dm *DBManager) KVRead(req DBRequest) (string, error) {
	db, err := dm.openConn(req, req.DBName)
	if err != nil {
		return "", err
	}
	defer db.Close()

	driver, _ := dm.buildDSN(req, req.DBName)
	if err := dm.ensureKVTable(db, driver); err != nil {
		return "", err
	}

	key := strings.TrimSpace(req.Path)
	if key == "" {
		// return all keys
		var query string
		if driver == "postgres" {
			query = "SELECT key, value FROM node_kv ORDER BY updated_at DESC LIMIT 50;"
		} else {
			query = "SELECT `key`, `value` FROM node_kv ORDER BY updated_at DESC LIMIT 50;"
		}
		rows, err := db.Query(query)
		if err != nil {
			return "", err
		}
		defer rows.Close()

		results := make(map[string]interface{})
		for rows.Next() {
			var k, v string
			if err := rows.Scan(&k, &v); err == nil {
				var jsonVal interface{}
				if json.Unmarshal([]byte(v), &jsonVal) == nil {
					results[k] = jsonVal
				} else {
					results[k] = v
				}
			}
		}
		out, _ := json.MarshalIndent(results, "", "  ")
		return string(out), nil
	}

	var query string
	if driver == "postgres" {
		query = "SELECT value FROM node_kv WHERE key = $1 LIMIT 1;"
	} else {
		query = "SELECT `value` FROM node_kv WHERE `key` = ? LIMIT 1;"
	}

	var val string
	err = db.QueryRow(query, key).Scan(&val)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("ключ '%s' не найден в базе '%s'", key, req.DBName)
		}
		return "", err
	}
	return val, nil
}

// KVWrite writes or merges value for specified path/key
func (dm *DBManager) KVWrite(req DBRequest) error {
	db, err := dm.openConn(req, req.DBName)
	if err != nil {
		return err
	}
	defer db.Close()

	driver, _ := dm.buildDSN(req, req.DBName)
	if err := dm.ensureKVTable(db, driver); err != nil {
		return err
	}

	key := strings.TrimSpace(req.Path)
	if key == "" {
		key = "default"
	}

	newValue := req.Value
	if req.Merge {
		// Fetch existing
		var existing string
		var q string
		if driver == "postgres" {
			q = "SELECT value FROM node_kv WHERE key = $1;"
		} else {
			q = "SELECT `value` FROM node_kv WHERE `key` = ?;"
		}
		if err := db.QueryRow(q, key).Scan(&existing); err == nil {
			var curMap, newMap map[string]interface{}
			if json.Unmarshal([]byte(existing), &curMap) == nil && json.Unmarshal([]byte(newValue), &newMap) == nil {
				for k, v := range newMap {
					curMap[k] = v
				}
				mergedBytes, _ := json.MarshalIndent(curMap, "", "  ")
				newValue = string(mergedBytes)
			} else {
				newValue = existing + "\n" + newValue
			}
		}
	}

	var upsertQuery string
	if driver == "postgres" {
		upsertQuery = `INSERT INTO node_kv (key, value, updated_at)
			VALUES ($1, $2, NOW())
			ON CONFLICT (key) DO UPDATE
			SET value = EXCLUDED.value, updated_at = NOW();`
		_, err = db.Exec(upsertQuery, key, newValue)
	} else {
		upsertQuery = `INSERT INTO node_kv (` + "`key`" + `, ` + "`value`" + `, updated_at)
			VALUES (?, ?, NOW())
			ON DUPLICATE KEY UPDATE ` + "`value`" + ` = VALUES(` + "`value`" + `), updated_at = NOW();`
		_, err = db.Exec(upsertQuery, key, newValue)
	}
	return err
}

// KVDelete deletes key or drops table/db if force
func (dm *DBManager) KVDelete(req DBRequest) error {
	db, err := dm.openConn(req, req.DBName)
	if err != nil {
		return err
	}
	defer db.Close()

	driver, _ := dm.buildDSN(req, req.DBName)
	key := strings.TrimSpace(req.Path)

	if key == "" && req.Force {
		// drop node_kv
		_, err = db.Exec("DROP TABLE IF EXISTS node_kv;")
		return err
	}

	var query string
	if driver == "postgres" {
		query = "DELETE FROM node_kv WHERE key = $1;"
	} else {
		query = "DELETE FROM node_kv WHERE `key` = ?;"
	}
	_, err = db.Exec(query, key)
	return err
}
