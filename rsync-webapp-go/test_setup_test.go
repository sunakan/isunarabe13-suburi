package main

import (
	"fmt"
	"os"
	"testing"
)

// TestMain関数は必ず最初に実行される
// グローバル変数等を埋める
func TestMain(m *testing.M) {
	fallbackImageHash = "d9f8294e9d895f81ce62e73dc7d5dff862a4fa40bd4e0fecf53f7526a8edcac0"
	err := os.Setenv("ISUCON13_MYSQL_DIALCONFIG_ADDRESS", "127.0.0.1")
	if err != nil {
		panic(err)
	}
	err = os.Setenv("ISUCON13_MYSQL_DIALCONFIG_PORT", "3306")
	if err != nil {
		panic(err)
	}

	// DB接続
	dbConn, err := connectDB(nil)
	if err != nil {
		fmt.Printf("DB接続に失敗しました: %+v\n", err)
		os.Exit(1)
	}
	defer dbConn.Close()

	// タグキャッシュのセットアップ
	initializeTagCache()

	// テストの実行
	code := m.Run()
	os.Exit(code)
}
