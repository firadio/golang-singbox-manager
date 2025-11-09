package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	db, err := sql.Open("sqlite3", "data/manager.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// 查询所有节点
	rows, err := db.Query("SELECT id, name, tun_name, tun_address FROM proxy_nodes")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	fmt.Println("Current nodes:")
	for rows.Next() {
		var id int
		var name, tunName, tunAddress string
		rows.Scan(&id, &name, &tunName, &tunAddress)
		fmt.Printf("  ID: %d, Name: %s, TUN: %s, Address: %s\n", id, name, tunName, tunAddress)
	}

	// 更新节点的 TUN 配置
	_, err = db.Exec(`
		UPDATE proxy_nodes
		SET tun_name = 'tun' || id,
		    tun_address = '172.18.0.' || ((id - 1) * 4 + 1) || '/30'
		WHERE tun_name = '' OR tun_name IS NULL
	`)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("\nFixed nodes:")
	rows, err = db.Query("SELECT id, name, tun_name, tun_address FROM proxy_nodes")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var name, tunName, tunAddress string
		rows.Scan(&id, &name, &tunName, &tunAddress)
		fmt.Printf("  ID: %d, Name: %s, TUN: %s, Address: %s\n", id, name, tunName, tunAddress)
	}

	fmt.Println("\nDone! Please restart the node to apply changes.")
}
