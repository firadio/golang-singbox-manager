#!/bin/bash
# 查看日志

if [ "$1" == "manager" ]; then
    echo "=== Manager Logs (last 50 lines) ==="
    journalctl -u singbox-manager -n 50 --no-pager
elif [ "$1" == "follow" ]; then
    echo "=== Following Manager Logs (Ctrl+C to exit) ==="
    journalctl -u singbox-manager -f
elif [ "$1" == "node" ]; then
    if [ -z "$2" ]; then
        echo "Usage: $0 node <node_id>"
        exit 1
    fi
    echo "=== Sing-box Node $2 Logs ==="
    tail -f logs/singbox/node_$2.log
else
    echo "Usage:"
    echo "  $0 manager         - Show last 50 lines of manager logs"
    echo "  $0 follow          - Follow manager logs in real-time"
    echo "  $0 node <id>       - Follow sing-box node logs"
    echo ""
    echo "Example:"
    echo "  $0 manager"
    echo "  $0 follow"
    echo "  $0 node 1"
fi
