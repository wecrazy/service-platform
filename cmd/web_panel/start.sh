#!/bin/bash

# Start MariaDB (MySQL-compatible)
service mariadb start

# Wait for MySQL to be ready
until mysqladmin ping >/dev/null 2>&1; do
  echo "Waiting for MySQL..."
  sleep 1
done

# Create user
mysql -u root -e "CREATE USER IF NOT EXISTS 'swi'@'localhost' IDENTIFIED BY 'Takasitau';"
mysql -u root -e "GRANT ALL PRIVILEGES ON *.* TO 'swi'@'localhost';"
mysql -u root -e "FLUSH PRIVILEGES;"

# Start Redis
service redis-server start

# Run the app
exec ./web_panel