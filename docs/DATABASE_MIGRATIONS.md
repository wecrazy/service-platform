# Database Migrations Guide

This document provides comprehensive information about the custom database migration system implemented in the Service Platform.

## Overview

The Service Platform uses a custom migration system that leverages GORM's AutoMigrate functionality within version-controlled migrations. This approach ensures:

- **Exact GORM Behavior**: Uses GORM's AutoMigrate for consistent column names, types, and constraints
- **Version Control**: All schema changes are tracked and versioned
- **Rollback Support**: Ability to rollback migrations safely
- **Environment Safety**: Prevents accidental schema changes in production
- **Team Collaboration**: Migrations can be reviewed and tested before deployment

## Architecture

### Migration Registry
- **Location**: `internal/migrations/registry.go`
- **Purpose**: Manages migration registration and execution
- **Features**: Automatic sorting, status tracking, rollback support

### Migration Files
- **Location**: `internal/database/migrations/` (registered via `init()` functions)
- **Naming**: `001_description.go`, `002_description.go`, etc.
- **Structure**: Each migration is a Go file with Up/Down functions

### Migration CLI
- **Location**: `cmd/migrate/main.go`
- **Purpose**: Standalone tool for running migrations
- **Features**: Up, down, status, and reset operations

## Migration Structure

Each migration file follows this structure:

```go
package migrations

import (
    "service-platform/internal/core/model"
    "time"
    "gorm.io/gorm"
)

func init() {
    RegisterMigration(&Migration{
        ID:        "001_description",
        Timestamp: time.Date(2025, 12, 30, 10, 51, 0, 0, time.UTC),
        Up:        upDescription,
        Down:      downDescription,
    })
}

func upDescription(db *gorm.DB) error {
    // Migration logic using GORM AutoMigrate or raw SQL
    return db.AutoMigrate(&model.NewTable{})
}

func downDescription(db *gorm.DB) error {
    // Rollback logic
    return db.Migrator().DropTable("new_table")
}
```

## Commands

### Running Migrations

```bash
# Run all pending migrations
make migrate-up

# Check migration status
make migrate-status

# Rollback last migration
make migrate-down

# Reset all migrations (WARNING: drops all tables)
make migrate-reset
```

### CLI Usage

```bash
# Build migration tool
make build-migrate

# Run migrations
./bin/migrate -action up

# Rollback specific number of migrations
./bin/migrate -action down -steps 2

# Check status
./bin/migrate -action status

# Reset all (destructive)
./bin/migrate -action reset
```

## Current Migrations

| Migration | Description | Status |
|-----------|-------------|--------|
| 001_initial_schema | Creates all database tables using GORM AutoMigrate | ✅ Active |
| 002_initial_seed | Seeds initial data (roles, users, features, etc.) | ✅ Active |

## How It Works

### Migration Execution
1. **Registration**: Migrations register themselves via `init()` functions
2. **Sorting**: Migrations are sorted by ID for consistent execution order
3. **Tracking**: Applied migrations are tracked in `schema_migrations` table
4. **Execution**: Only pending migrations are executed
4. **Recording**: Successful migrations are recorded to prevent re-execution

### Schema Creation
- Uses GORM's `AutoMigrate()` for exact column names and types
- Respects GORM struct tags (`gorm:"column:name"`, etc.)
- Handles database-specific differences automatically
- Maintains consistency with existing GORM configuration

### Data Seeding
- Integrated into migrations for version control
- Uses existing seeding functions
- Ensures data consistency across environments

## Best Practices

### Schema Changes

1. **Always provide rollback**: Every `up` migration must have a corresponding `down` migration
2. **Use GORM AutoMigrate**: Leverage GORM's migration capabilities for consistency
3. **Test migrations**: Test on staging environment before production
4. **Small, focused changes**: Keep migrations small and focused on single changes
5. **Document changes**: Comment migration logic clearly

### Data Migrations

1. **Backup first**: Always backup data before running data migrations
2. **Handle dependencies**: Consider foreign key constraints during rollbacks
3. **Validate data**: Add validation checks in migrations
4. **Use transactions**: Migrations run within database transactions

### Development Workflow

1. **Create migration**: Add new migration file with sequential ID
2. **Implement logic**: Use GORM AutoMigrate or raw SQL as needed
3. **Test locally**: Run migrations on development database
4. **Commit together**: Commit migration with related code changes
5. **Deploy carefully**: Test migrations on staging before production

## Examples

### Adding a New Model

```go
// 003_add_user_preferences.go
func upAddUserPreferences(db *gorm.DB) error {
    return db.AutoMigrate(&model.UserPreferences{})
}

func downAddUserPreferences(db *gorm.DB) error {
    return db.Migrator().DropTable("user_preferences")
}
```

### Adding a Column

```go
// 004_add_user_avatar.go
func upAddUserAvatar(db *gorm.DB) error {
    return db.Migrator().AddColumn(&model.Users{}, "avatar_url")
}

func downAddUserAvatar(db *gorm.DB) error {
    return db.Migrator().DropColumn(&model.Users{}, "avatar_url")
}
```

### Data Migration

```go
// 005_migrate_user_data.go
func upMigrateUserData(db *gorm.DB) error {
    return db.Model(&model.Users{}).Where("avatar_url IS NULL").Update("avatar_url", "default.jpg").Error
}

func downMigrateUserData(db *gorm.DB) error {
    // Data rollbacks are often not possible
    return nil
}
```

## Migration Status Table

The system uses a `schema_migrations` table to track applied migrations:

```sql
CREATE TABLE schema_migrations (
    id VARCHAR(255) PRIMARY KEY,
    applied_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
```

## Environment Configuration

### Development
- Uses `internal/config/service-platform.dev.yaml` configuration
- Allows all migration operations
- Suitable for rapid development

### Production
- Uses `internal/config/service-platform.prod.yaml` configuration
- Should have additional safeguards
- Requires careful testing before production

## Troubleshooting

### Migration Fails

1. **Check database connectivity**: Ensure database is accessible
2. **Verify migration syntax**: Check Go code for errors
3. **Review dependencies**: Check for circular dependencies
4. **Check permissions**: Ensure user has DDL permissions

### Rollback Issues

1. **Foreign key constraints**: May prevent table drops
2. **Data dependencies**: Existing data may reference migrated structures
3. **Manual cleanup**: May require manual intervention

### Common Issues

- **Migration not registered**: Ensure `init()` function calls `RegisterMigration()`
- **ID conflicts**: Use unique, sequential IDs
- **GORM version differences**: Test with same GORM version as production

## Migration Workflow

### Development

1. **Create feature branch**
2. **Make code changes** (add models, modify structs)
3. **Create migration**: Add new migration file
4. **Implement migration logic** using GORM AutoMigrate
5. **Test migration**: Run on development database
6. **Test rollback**: Ensure down migration works
7. **Commit changes**: Include migration and code changes

### Deployment

1. **Deploy code changes**
2. **Run migrations**: Execute `make migrate-up`
3. **Verify functionality**: Test application features
4. **Monitor issues**: Watch for migration-related errors

### Emergency Rollback

1. **Identify problematic migration**
2. **Rollback if safe**: Use `make migrate-down`
3. **Fix issues**: Address root cause
4. **Re-deploy**: Run migrations again after fixes

## Integration with Application

### Automatic Migration on Startup

The application automatically runs migrations on startup via `AutoMigrateDB()`:

```go
func AutoMigrateDB(db *gorm.DB) {
    if err := migrations.RunMigrations(db); err != nil {
        logrus.Fatalf("error while running database migrations: %v", err)
    }
}
```

### Manual Migration Management

Use the CLI tool for manual migration management:

```bash
# Status check
make migrate-status

# Apply pending migrations
make migrate-up

# Rollback last migration
make migrate-down
```

## Future Enhancements

- **Migration templates**: Standardized migration file templates
- **Validation**: Automatic migration validation
- **Dry-run mode**: Preview migration changes without executing
- **Branch-specific migrations**: Environment-specific migration paths
- **Migration dependencies**: Support for migration prerequisites

## References

- [GORM Migration Documentation](https://gorm.io/docs/migration.html)
- [PostgreSQL DDL Best Practices](https://www.postgresql.org/docs/current/ddl.html)
- [Database Migration Patterns](https://martinfowler.com/articles/evodb.html)

---

If you missing file database/db_region_indonesia.sql, you can fetch from : https://github.com/SanjayaDev/wilayah-indonesia-sql/blob/main/db_region_indonesia.sql