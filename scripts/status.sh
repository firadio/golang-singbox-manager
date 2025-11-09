#!/bin/bash
# 查看 singbox-manager 状态

echo "=== Singbox Manager Status ==="
systemctl status singbox-manager --no-pager

echo ""
echo "=== API Health Check ==="
curl -s http://localhost:8080/health

echo ""
echo ""
echo "=== System Info ==="
curl -s http://localhost:8080/api/system/info | python3 -m json.tool 2>/dev/null

echo ""
echo "=== All Nodes ==="
curl -s http://localhost:8080/api/nodes | python3 -m json.tool 2>/dev/null

echo ""
echo "=== All Rules ==="
curl -s http://localhost:8080/api/rules | python3 -m json.tool 2>/dev/null

echo ""
echo "=== Running Sing-box Processes ==="
ps aux | grep sing-box | grep -v grep || echo "No sing-box processes running"
