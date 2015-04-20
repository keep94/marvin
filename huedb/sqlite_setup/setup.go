// Package sqlite_setup sets up a sqlite database for Hue Web App
package sqlite_setup

import (
  "code.google.com/p/gosqlite/sqlite"
)

// SetUpTables creates all needed tables in database.
func SetUpTables(conn *sqlite.Conn) error {
  err := conn.Exec("create table if not exists named_colors (id INTEGER PRIMARY KEY AUTOINCREMENT, description TEXT, colors TEXT)")
  if err != nil {
    return err
  }
  err = conn.Exec("create table if not exists at_time_tasks (id INTEGER PRIMARY KEY AUTOINCREMENT, schedule_id TEXT, hue_task_id INTEGER, action TEXT, description TEXT, light_set TEXT, time INTEGER)")
  if err != nil {
    return err
  }
  err = conn.Exec("create index if not exists at_time_tasks_scheduleid_idx on at_time_tasks (schedule_id)")
  if err != nil {
    return err
  }
  return nil
}
