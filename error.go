package main

import (
	"bitbucket.org/mjl/sherpa"
	"database/sql"
	"encoding/json"
	"log"
	"runtime/debug"

	"github.com/lib/pq"
)

func sherpaCheck(err error, msg string) {
	if err == nil {
		return
	}

	if pqe, ok := err.(*pq.Error); ok && !config.ShowSherpaErrors {
		switch pqe.Code {
		case "23503":
			userError("References to this object still present in database.")
		case "23505":
			userError("Values are not unique.")
		case "23514":
			userError("Invalid value(s).")
		}
	}

	m := msg
	if m != "" {
		m += ": "
	}
	m += err.Error()
	if config.PrintSherpaErrorStack {
		log.Println("sherpa serverError:", m)
		debug.PrintStack()
	}
	if config.ShowSherpaErrors {
		m = msg + ": " + err.Error()
	} else {
		m = "An error occurred. Please try again later or contact us."
	}
	serverError(m)
}

func serverError(m string) {
	panic(&sherpa.Error{Code: "serverError", Message: m})
}

func sherpaUserCheck(err error, msg string) {
	if err == nil {
		return
	}

	m := msg
	if m != "" {
		m += ": "
	}
	m += err.Error()
	if false && config.PrintSherpaErrorStack {
		log.Println("sherpa userError:", m)
		debug.PrintStack()
	}
	if config.ShowSherpaErrors {
		m = msg + ": " + err.Error()
	} else {
		m = "An error occurred. Please try again later or contact us."
	}
	userError(m)
}

func userError(m string) {
	panic(&sherpa.Error{Code: "userError", Message: m})
}

func sherpaCheckRow(row *sql.Row, r interface{}, msg string) {
	var buf []byte
	err := row.Scan(&buf)
	if err == sql.ErrNoRows {
		panic(&sherpa.Error{Code: "userNotFound", Message: "Not found"})
	}
	sherpaCheck(err, msg+": reading json from database row into buffer")
	sherpaCheck(json.Unmarshal(buf, r), msg+": parsing json from database")
}

func checkRow(row *sql.Row, r interface{}, msg string) {
	var buf []byte
	err := row.Scan(&buf)
	if err == sql.ErrNoRows {
		log.Fatal("no row in result")
	}
	check(err, msg+": reading json from database row into buffer")
	check(json.Unmarshal(buf, r), msg+": parsing json from database")
}
