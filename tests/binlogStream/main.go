package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"os/signal"

	"github.com/onlyac0611/binlog"
)

func main() {
	file, err := os.OpenFile("transaction.txt", os.O_RDWR|os.O_CREATE|os.O_APPEND, os.ModePerm)
	if err != nil {
		log.Fatalf("OpenFile fail. err: %v", err)
		return
	}
	defer file.Close()

	log.SetOutput(file)
	log.SetFlags(log.Lmicroseconds | log.LstdFlags | log.Lshortfile)
	binlog.SetLogger(binlog.NewDefaultLogger(os.Stdout, binlog.InfoLevel))
	dsn := "example:example@tcp(localhost:3306)/mysql?charset=utf8mb4"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("open fail. err: %v", err)
		return
	}
	defer db.Close()

	db.SetMaxIdleConns(2)
	db.SetMaxOpenConns(4)

	e := &exampleMysqlTableMapper{db: db}
	format, err := e.GetBinlogFormat()
	if err != nil {
		log.Fatalf("getBinlogFormat fail. err: %v", err)
		return
	}

	if !format.IsRow() {
		log.Fatalf("binlog format is not row. format: %v", format)
		return
	}

	pos, err := e.GetBinlogPosition()
	if err != nil {
		log.Fatalf("GetBinlogPosition fail. err: %v", err)
		return
	}

	r, err := binlog.NewRowStreamer(dsn, 1234, e)
	if err != nil {
		log.Fatalf("NewRowStreamer fail. err: %v", err)
		return
	}
	r.SetStartBinlogPosition(pos)

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	processWait := make(chan os.Signal, 1)
	signal.Notify(processWait, os.Kill, os.Interrupt)

	go func() {
		select {
		case <-processWait:
			cancel()
		}
	}()

	err = r.Stream(ctx, func(t *binlog.Transaction) error {
		showTransaction(t, file)
		return nil
	})
}
