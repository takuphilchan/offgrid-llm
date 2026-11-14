package agents

import (
	"database/sql"
	"fmt"
	"strings"
)

func migrateControlRecordKinds(tx *sql.Tx) error {
	var definition string
	if err := tx.QueryRow(`SELECT sql FROM sqlite_schema WHERE name='agent_control_records' AND type='table'`).Scan(&definition); err != nil {
		return err
	}
	if strings.Contains(definition, "'user_preferences'") {
		return nil
	}
	// Preserve explicit indexes/triggers; the primary key is recreated by DDL.
	rows, err := tx.Query(`SELECT sql FROM sqlite_schema WHERE tbl_name='agent_control_records' AND type IN ('index','trigger') AND sql IS NOT NULL ORDER BY type,name`)
	if err != nil {
		return err
	}
	var definitions []string
	for rows.Next() {
		var sqlText string
		if err = rows.Scan(&sqlText); err != nil {
			rows.Close()
			return err
		}
		definitions = append(definitions, sqlText)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, statement := range []string{
		`CREATE TABLE agent_control_records_v7 ` + controlRecordColumns,
		`INSERT INTO agent_control_records_v7 SELECT workspace,actor,id,kind,revision,event_floor,snapshot FROM agent_control_records`,
		`DROP TABLE agent_control_records`,
		`ALTER TABLE agent_control_records_v7 RENAME TO agent_control_records`,
	} {
		if _, err = tx.Exec(statement); err != nil {
			return err
		}
	}
	for _, definition := range definitions {
		if _, err = tx.Exec(definition); err != nil {
			return err
		}
	}
	return nil
}

func checkControlForeignKeys(tx *sql.Tx) error {
	rows, err := tx.Query(`PRAGMA foreign_key_check`)
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		return fmt.Errorf("task migration would retain invalid foreign keys")
	}
	return rows.Err()
}
