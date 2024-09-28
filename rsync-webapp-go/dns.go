package main

import (
	"fmt"
	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"net"
	"strings"
)

var domainId int64

// connectDNSDB は、DNS用のDBへのコネクションを張り、返す関数です
func connectDNSDB(dbIpAddress string) (*sqlx.DB, error) {
	conf := mysql.NewConfig()
	conf.Net = "tcp"
	conf.Addr = net.JoinHostPort(dbIpAddress, "3306")
	conf.User = "isucon"
	conf.Passwd = "isucon"
	conf.DBName = "isudns"
	conf.ParseTime = true
	conf.InterpolateParams = true

	db, err := sqlx.Open("mysql", conf.FormatDSN())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)

	if err := db.Ping(); err != nil {
		return nil, err
	}

	type DomainId struct {
		Value int64 `db:"domain_id"`
	}
	var d DomainId
	if err := db.Get(&d, "SELECT domain_id FROM records ORDER BY id LIMIT 1;"); err != nil {
		fmt.Printf("DNS DB: クエリ失敗: %+v\n", err)
		return nil, err
	}
	domainId = d.Value

	return db, nil
}

// kaizen-05: DNS用のDBに直接サブドメインを追加
// addSubdomain は、サブドメインをDNS用のDBに追加する
func addSubdomain(subdomain string) error {
	if _, err := dnsDbConn.Exec("INSERT INTO records (domain_id, name, type, content, ttl, prio, disabled, ordername, auth) VALUES(?, ?, 'A', '192.168.0.12', 120, 0, 0, NULL, 1)", domainId, strings.ToLower(subdomain)+".t.isucon.pw"); err != nil {
		fmt.Printf("DNS DB: クエリ失敗: %+v\n", err)
		return err
	}
	return nil
}
