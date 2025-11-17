#!/usr/bin/env python3
"""
Update existing nodes to add HTTP and SOCKS5 ports
"""
import sqlite3
import sys

def update_node_ports(db_path):
    conn = sqlite3.connect(db_path)
    cursor = conn.cursor()

    # Get all nodes
    cursor.execute("SELECT id, http_port, socks5_port FROM proxy_nodes")
    nodes = cursor.fetchall()

    updated = 0
    for node_id, http_port, socks5_port in nodes:
        # Only update if ports are NULL
        if http_port is None or socks5_port is None:
            new_http_port = 8000 + node_id
            new_socks5_port = 5000 + node_id

            cursor.execute(
                "UPDATE proxy_nodes SET http_port = ?, socks5_port = ? WHERE id = ?",
                (new_http_port, new_socks5_port, node_id)
            )
            print(f"Updated node {node_id}: HTTP port {new_http_port}, SOCKS5 port {new_socks5_port}")
            updated += 1

    conn.commit()
    conn.close()

    print(f"\nTotal nodes updated: {updated}")
    return updated

if __name__ == "__main__":
    db_path = "/root/github.com/firadio/golang-singbox-manager/data/manager.db"
    if len(sys.argv) > 1:
        db_path = sys.argv[1]

    print(f"Updating node ports in database: {db_path}")
    update_node_ports(db_path)
    print("Done! Please restart nodes to apply changes.")
