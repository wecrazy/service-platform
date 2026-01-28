#!/bin/bash

# Service Platform Telegram Service Installer
# This script installs the Telegram bot as a systemd service

set -e

# Configuration
SERVICE_NAME="service-platform-telegram"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"
BINARY_PATH="$(pwd)/bin/telegram"
WORKING_DIR="$(pwd)"
USER_NAME="service-platform"
GROUP_NAME="service-platform"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if running as root
check_root() {
    if [[ $EUID -ne 0 ]]; then
        log_error "This script must be run as root (sudo)"
        exit 1
    fi
}

# Check if service already exists
check_existing_service() {
    if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
        log_warning "Service '$SERVICE_NAME' is already active"
        read -p "Do you want to restart it? (y/N): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            log_info "Restarting service..."
            systemctl restart "$SERVICE_NAME"
            log_success "Service restarted successfully"
            exit 0
        else
            log_info "Installation cancelled"
            exit 0
        fi
    fi

    if systemctl is-enabled --quiet "$SERVICE_NAME" 2>/dev/null; then
        log_warning "Service '$SERVICE_NAME' is already installed but not running"
        read -p "Do you want to start it? (y/N): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            log_info "Starting service..."
            systemctl start "$SERVICE_NAME"
            log_success "Service started successfully"
            exit 0
        else
            log_info "Installation cancelled"
            exit 0
        fi
    fi
}

# Check if binary exists
check_binary() {
    if [[ ! -f "$BINARY_PATH" ]]; then
        log_error "Telegram binary not found at $BINARY_PATH"
        log_info "Please build the Telegram service first:"
        log_info "  make build-telegram"
        exit 1
    fi
}

# Create service user if it doesn't exist
create_service_user() {
    if ! id "$USER_NAME" &>/dev/null; then
        log_info "Creating service user '$USER_NAME'..."
        useradd --system --shell /bin/false --home-dir "$WORKING_DIR" --create-home "$USER_NAME" 2>/dev/null || true
        log_success "Service user created"
    else
        log_info "Service user '$USER_NAME' already exists"
    fi

    # Ensure the user owns the working directory
    chown -R "$USER_NAME:$GROUP_NAME" "$WORKING_DIR" 2>/dev/null || true
}

# Create systemd service file
create_service_file() {
    log_info "Creating systemd service file..."

    cat > "$SERVICE_FILE" << SERVICE_EOF
[Unit]
Description=Service Platform Telegram Bot
After=network.target
Wants=network.target

[Service]
Type=simple
User=$USER_NAME
Group=$GROUP_NAME
WorkingDirectory=$WORKING_DIR
ExecStart=$BINARY_PATH
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=$SERVICE_NAME

# Environment variables
Environment=CONFIG_FILE=$WORKING_DIR/internal/config/config.yaml

# Security settings
NoNewPrivileges=yes
PrivateTmp=yes
ProtectSystem=strict
ReadWritePaths=$WORKING_DIR

[Install]
WantedBy=multi-user.target
SERVICE_EOF

    log_success "Service file created at $SERVICE_FILE"
}

# Enable and start service
enable_and_start_service() {
    log_info "Reloading systemd daemon..."
    systemctl daemon-reload

    log_info "Enabling service..."
    systemctl enable "$SERVICE_NAME"

    log_info "Starting service..."
    systemctl start "$SERVICE_NAME"

    log_info "Checking service status..."
    sleep 2
    if systemctl is-active --quiet "$SERVICE_NAME"; then
        log_success "Service installed and started successfully!"
        echo
        log_info "Service management commands:"
        echo "  sudo systemctl status $SERVICE_NAME    # Check status"
        echo "  sudo systemctl stop $SERVICE_NAME      # Stop service"
        echo "  sudo systemctl start $SERVICE_NAME     # Start service"
        echo "  sudo systemctl restart $SERVICE_NAME   # Restart service"
        echo "  sudo systemctl enable $SERVICE_NAME    # Enable auto-start"
        echo "  sudo systemctl disable $SERVICE_NAME   # Disable auto-start"
        echo
        log_info "View logs with:"
        echo "  sudo journalctl -u $SERVICE_NAME -f"
    else
        log_error "Service failed to start. Check logs:"
        echo "  sudo journalctl -u $SERVICE_NAME -n 50"
        exit 1
    fi
}

# Main installation function
install_service() {
    log_info "Installing Service Platform Telegram Service..."

    check_root
    check_existing_service
    check_binary
    create_service_user
    create_service_file
    enable_and_start_service

    log_success "Installation completed!"
}

# Uninstall function
uninstall_service() {
    log_info "Uninstalling Service Platform Telegram Service..."

    check_root

    if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
        log_info "Stopping service..."
        systemctl stop "$SERVICE_NAME" || true
    fi

    if systemctl is-enabled --quiet "$SERVICE_NAME" 2>/dev/null; then
        log_info "Disabling service..."
        systemctl disable "$SERVICE_NAME" || true
    fi

    if [[ -f "$SERVICE_FILE" ]]; then
        log_info "Removing service file..."
        rm -f "$SERVICE_FILE"
        systemctl daemon-reload
    fi

    # Optionally remove user (commented out for safety)
    # if id "$USER_NAME" &>/dev/null; then
    #     log_info "Removing service user..."
    #     userdel "$USER_NAME" 2>/dev/null || true
    # fi

    log_success "Service uninstalled successfully!"
}

# Show usage
show_usage() {
    echo "Service Platform Telegram Service Installer"
    echo
    echo "Usage:"
    echo "  sudo $0 install    # Install and start the Telegram service"
    echo "  sudo $0 uninstall  # Stop and remove the Telegram service"
    echo "  sudo $0 status     # Show service status"
    echo
    echo "Requirements:"
    echo "  - Run as root (sudo)"
    echo "  - Telegram binary must be built (make build-telegram)"
    echo "  - Configuration file must exist"
}

# Main script logic
case "${1:-}" in
    install)
        install_service
        ;;
    uninstall)
        uninstall_service
        ;;
    status)
        if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
            log_success "Service is running"
            systemctl status "$SERVICE_NAME" --no-pager
        elif systemctl is-enabled --quiet "$SERVICE_NAME" 2>/dev/null; then
            log_warning "Service is installed but not running"
            systemctl status "$SERVICE_NAME" --no-pager
        else
            log_info "Service is not installed"
        fi
        ;;
    *)
        show_usage
        exit 1
        ;;
esac
