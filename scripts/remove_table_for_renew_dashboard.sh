#!/bin/bash

# Script to empty dashboard-related tables from the database
# This script reads configuration from conf.yaml and the appropriate config file

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
CONFIG_DIR="$PROJECT_ROOT/internal/config"

echo -e "${GREEN}=== Dashboard Table Cleanup Script ===${NC}"
echo ""

# Function to parse YAML values (simple key-value parser)
parse_yaml() {
    local file=$1
    local key=$2
    
    # Remove leading dot from key if present
    key=${key#.}
    
    # Handle nested keys (e.g., database.type)
    if [[ $key == *.* ]]; then
        # For nested keys, we need to find the section first
        local section=${key%%.*}
        local subkey=${key#*.}
        
        # Extract value from nested structure
        awk -v section="$section:" -v key="$subkey:" '
            $0 ~ "^"section {in_section=1; next}
            in_section && /^[a-zA-Z]/ && $0 !~ "^  " {in_section=0}
            in_section && $0 ~ "^  "key {
                gsub(/^  [^:]*: *"?/, "")
                gsub(/"? *#.*$/, "")
                gsub(/"$/, "")
                print
                exit
            }
        ' "$file"
    else
        # For top-level keys
        awk -v key="$key:" '
            $0 ~ "^"key {
                gsub(/^[^:]*: *"?/, "")
                gsub(/"? *#.*$/, "")
                gsub(/"$/, "")
                print
                exit
            }
        ' "$file"
    fi
}

# Step 1: Read config mode from conf.yaml
echo -e "${YELLOW}[1/5] Reading configuration mode...${NC}"
CONF_FILE="$CONFIG_DIR/conf.yaml"

if [ ! -f "$CONF_FILE" ]; then
    echo -e "${RED}Error: Configuration file not found: $CONF_FILE${NC}"
    exit 1
fi

CONFIG_MODE=$(parse_yaml "$CONF_FILE" "config_mode")
echo -e "Config mode: ${GREEN}$CONFIG_MODE${NC}"

# Step 2: Determine which config file to use
echo -e "${YELLOW}[2/5] Loading configuration file...${NC}"
CONFIG_FILE="$CONFIG_DIR/service-platform.$CONFIG_MODE.yaml"

if [ ! -f "$CONFIG_FILE" ]; then
    echo -e "${RED}Error: Configuration file not found: $CONFIG_FILE${NC}"
    exit 1
fi

echo -e "Using config: ${GREEN}$CONFIG_FILE${NC}"

# Step 3: Extract database configuration
echo -e "${YELLOW}[3/5] Extracting database configuration...${NC}"

DB_TYPE=$(parse_yaml "$CONFIG_FILE" "database.type")
DB_HOST=$(parse_yaml "$CONFIG_FILE" "database.host")
DB_PORT=$(parse_yaml "$CONFIG_FILE" "database.port")
DB_USER=$(parse_yaml "$CONFIG_FILE" "database.username")
DB_PASS=$(parse_yaml "$CONFIG_FILE" "database.password")
DB_NAME=$(parse_yaml "$CONFIG_FILE" "database.name")

# Extract table names
TB_ROLE=$(parse_yaml "$CONFIG_FILE" "database.tb_role")
TB_ROLE_PRIVILEGE=$(parse_yaml "$CONFIG_FILE" "database.tb_role_privilege")
TB_FEATURE=$(parse_yaml "$CONFIG_FILE" "database.tb_feature")

echo -e "Database Type: ${GREEN}$DB_TYPE${NC}"
echo -e "Database Host: ${GREEN}$DB_HOST:$DB_PORT${NC}"
echo -e "Database Name: ${GREEN}$DB_NAME${NC}"
echo -e "Tables to empty:"
echo -e "  - ${GREEN}$TB_ROLE${NC}"
echo -e "  - ${GREEN}$TB_ROLE_PRIVILEGE${NC}"
echo -e "  - ${GREEN}$TB_FEATURE${NC}"
echo ""

# Step 4: Confirm action
echo -e "${RED}WARNING: This will delete all data from the tables above!${NC}"
read -p "Are you sure you want to continue? (yes/no): " CONFIRM

if [ "$CONFIRM" != "yes" ]; then
    echo -e "${YELLOW}Operation cancelled.${NC}"
    exit 0
fi

# Step 5: Empty the tables
echo -e "${YELLOW}[4/5] Connecting to database and emptying tables...${NC}"

if [ "$DB_TYPE" = "PostgreSQL" ]; then
    # Check if psql is installed
    if ! command -v psql &> /dev/null; then
        echo -e "${RED}Error: 'psql' is not installed. Please install PostgreSQL client.${NC}"
        exit 1
    fi

    # PostgreSQL connection
    export PGPASSWORD="$DB_PASS"
    
    echo -e "Emptying table: ${GREEN}$TB_ROLE_PRIVILEGE${NC}..."
    psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "TRUNCATE TABLE \"$TB_ROLE_PRIVILEGE\" CASCADE;" || {
        echo -e "${RED}Failed to empty $TB_ROLE_PRIVILEGE${NC}"
        exit 1
    }
    
    echo -e "Emptying table: ${GREEN}$TB_ROLE${NC}..."
    psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "TRUNCATE TABLE \"$TB_ROLE\" CASCADE;" || {
        echo -e "${RED}Failed to empty $TB_ROLE${NC}"
        exit 1
    }
    
    echo -e "Emptying table: ${GREEN}$TB_FEATURE${NC}..."
    psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "TRUNCATE TABLE \"$TB_FEATURE\" CASCADE;" || {
        echo -e "${RED}Failed to empty $TB_FEATURE${NC}"
        exit 1
    }
    
    unset PGPASSWORD

elif [ "$DB_TYPE" = "MySQL" ]; then
    # Check if mysql is installed
    if ! command -v mysql &> /dev/null; then
        echo -e "${RED}Error: 'mysql' is not installed. Please install MySQL client.${NC}"
        exit 1
    fi

    # MySQL connection
    echo -e "Emptying table: ${GREEN}$TB_ROLE_PRIVILEGE${NC}..."
    mysql -h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER" -p"$DB_PASS" "$DB_NAME" -e "TRUNCATE TABLE \`$TB_ROLE_PRIVILEGE\`;" || {
        echo -e "${RED}Failed to empty $TB_ROLE_PRIVILEGE${NC}"
        exit 1
    }
    
    echo -e "Emptying table: ${GREEN}$TB_ROLE${NC}..."
    mysql -h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER" -p"$DB_PASS" "$DB_NAME" -e "TRUNCATE TABLE \`$TB_ROLE\`;" || {
        echo -e "${RED}Failed to empty $TB_ROLE${NC}"
        exit 1
    }
    
    echo -e "Emptying table: ${GREEN}$TB_FEATURE${NC}..."
    mysql -h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER" -p"$DB_PASS" "$DB_NAME" -e "TRUNCATE TABLE \`$TB_FEATURE\`;" || {
        echo -e "${RED}Failed to empty $TB_FEATURE${NC}"
        exit 1
    }

else
    echo -e "${RED}Error: Unsupported database type: $DB_TYPE${NC}"
    exit 1
fi

echo ""
echo -e "${YELLOW}[5/5]${NC} ${GREEN}✓ Successfully emptied all tables!${NC}"
echo -e "${GREEN}=== Operation completed successfully ===${NC}"
