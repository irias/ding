package main

import (
	"database/sql"
	"log"
)

func transact(fn func(tx *sql.Tx)) {
	tx, err := database.Begin()
	sherpaCheck(err, "starting database transaction")
	defer func() {
		if e := recover(); e != nil {
			defer func() {
				panic(e)
			}()

			ee := tx.Rollback()
			if ee != nil {
				log.Println("rolling back:", ee)
			}
		}
	}()
	fn(tx)
	err = tx.Commit()
	sherpaCheck(err, "committing database transaction")
}
