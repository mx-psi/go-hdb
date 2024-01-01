package dbtest

import (
	"database/sql"
	"fmt"
	"strconv"
)

// mSessionContext represents the hdb M_SESSION_CONTEXT system view.
type mSessionContext struct {
	host         string
	port         int
	connectionID int
	key          string
	value        string
	section      string
	// ddlEnabled   sql.NullInt64 // not always popuated (see HANA docu for m_session_context for reference).
}

func sessionContext(db *sql.DB) ([]mSessionContext, error) {
	rows, err := db.Query("select host, port, connection_id, key, value, section from m_session_context where connection_id=current_connection")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	mscs := []mSessionContext{}
	var msc mSessionContext

	for rows.Next() {
		if err := rows.Scan(&msc.host, &msc.port, &msc.connectionID, &msc.key, &msc.value, &msc.section); err != nil {
			return nil, err
		}
		mscs = append(mscs, msc)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return mscs, nil
}

// QuerySessionVariables returns a map of current session variables.
func QuerySessionVariables(db *sql.DB) (map[string]string, error) {
	mscs, err := sessionContext(db)
	if err != nil {
		return nil, err
	}
	sv := make(map[string]string, len(mscs))
	for _, v := range mscs {
		sv[v.key] = v.value
	}
	return sv, nil
}

// createSchema creates a schema on the database.
func createSchema(db *sql.DB, schema string) error {
	_, err := db.Exec(fmt.Sprintf("create schema %s", strconv.Quote(schema)))
	return err
}

// dropSchema drops a schema from the database.
func dropSchema(db *sql.DB, schema string) error {
	_, err := db.Exec(fmt.Sprintf("drop schema %s cascade", strconv.Quote(schema)))
	return err
}

// queryNumTablesInSchema returns the number of tables in a database schema.
func queryNumTablesInSchema(db *sql.DB, schema string) (int, error) {
	numTables := 0
	if err := db.QueryRow(fmt.Sprintf("select count(*) from sys.tables where schema_name = '%s'", schema)).Scan(&numTables); err != nil {
		return 0, err
	}
	return numTables, nil
}

// queryNumProcsInSchema returns the number of stored procedures in a database schema.
func queryNumProcsInSchema(db *sql.DB, schema string) (int, error) {
	numProcs := 0
	if err := db.QueryRow(fmt.Sprintf("select count(*) from sys.procedures where schema_name = '%s'", schema)).Scan(&numProcs); err != nil {
		return 0, err
	}
	return numProcs, nil
}

// querySchemasPrefix returns all schemas of a database starting with prefix in name.
func querySchemasPrefix(db *sql.DB, prefix string) ([]string, error) {
	names := make([]string, 0)

	rows, err := db.Query(fmt.Sprintf("select schema_name from sys.schemas where schema_name like '%s_%%'", prefix))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var name string
	for rows.Next() {
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return names, nil
}

// QueryInvalidConnectAttempts returns the number of invalid connection attempts.
func QueryInvalidConnectAttempts(db *sql.DB, username string) int64 {
	invalidConnectAttempts := int64(0)

	// ignore error (entry not found)
	db.QueryRow(fmt.Sprintf("select invalid_connect_attempts from sys.invalid_connect_attempts where user_name = '%s'", username)).Scan(&invalidConnectAttempts) //nolint:errcheck
	return invalidConnectAttempts
}