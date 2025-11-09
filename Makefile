.PHONY: build run clean install test

# 变量定义
BINARY_NAME=singbox-manager
INSTALL_PATH=/usr/local/bin/$(BINARY_NAME)
SERVICE_FILE=/etc/systemd/system/singbox-manager.service

# 构建
build:
	@echo "Building $(BINARY_NAME)..."
	@export PATH=$$PATH:/usr/local/go/bin && CGO_ENABLED=1 go build -o $(BINARY_NAME) cmd/manager/main.go
	@echo "Build complete: $(BINARY_NAME)"

# 运行
run: build
	@echo "Running $(BINARY_NAME)..."
	@sudo ./$(BINARY_NAME)

# 清理
clean:
	@echo "Cleaning..."
	@rm -f $(BINARY_NAME)
	@rm -rf data/*.db
	@rm -rf logs/*
	@echo "Clean complete"

# 安装
install: build
	@echo "Installing $(BINARY_NAME)..."
	@sudo cp $(BINARY_NAME) $(INSTALL_PATH)
	@sudo chmod +x $(INSTALL_PATH)
	@echo "Installed to $(INSTALL_PATH)"
	@echo ""
	@echo "Creating systemd service..."
	@sudo bash -c 'cat > $(SERVICE_FILE) <<EOF\n\
[Unit]\n\
Description=Golang Sing-Box Manager\n\
After=network.target\n\
\n\
[Service]\n\
Type=simple\n\
User=root\n\
WorkingDirectory=$(shell pwd)\n\
ExecStart=$(INSTALL_PATH) -config $(shell pwd)/configs/config.yaml\n\
Restart=on-failure\n\
RestartSec=5s\n\
\n\
[Install]\n\
WantedBy=multi-user.target\n\
EOF'
	@sudo systemctl daemon-reload
	@echo "Service installed: $(SERVICE_FILE)"
	@echo ""
	@echo "To start the service:"
	@echo "  sudo systemctl start singbox-manager"
	@echo "To enable auto-start:"
	@echo "  sudo systemctl enable singbox-manager"

# 卸载
uninstall:
	@echo "Uninstalling $(BINARY_NAME)..."
	@sudo systemctl stop singbox-manager 2>/dev/null || true
	@sudo systemctl disable singbox-manager 2>/dev/null || true
	@sudo rm -f $(SERVICE_FILE)
	@sudo rm -f $(INSTALL_PATH)
	@sudo systemctl daemon-reload
	@echo "Uninstall complete"

# 测试
test:
	@export PATH=$$PATH:/usr/local/go/bin && go test -v ./...

# 格式化代码
fmt:
	@export PATH=$$PATH:/usr/local/go/bin && go fmt ./...

# 查看日志
logs:
	@sudo journalctl -u singbox-manager -f

# 查看状态
status:
	@sudo systemctl status singbox-manager
