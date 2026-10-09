package migration

import "gorm.io/gorm"

// TableExists checks whether a table exists in the current database.
// Used to detect whether versioned migrations have been applied.
func TableExists(db *gorm.DB, tableName string) bool {
	var count int64
	db.Raw("SELECT count(*) FROM information_schema.tables WHERE table_name = ?", tableName).Scan(&count)
	return count > 0
}

// ShouldFallbackToAutoMigrate returns true if the schema_migrations table
// does not exist (i.e., versioned migrations have not been applied yet).
// In that case the caller should use GORM AutoMigrate as a fallback,
// preserving compatibility with existing databases.
func ShouldFallbackToAutoMigrate(db *gorm.DB) bool {
	return !TableExists(db, "schema_migrations")
}

// HasMigrationVersion checks whether a specific migration version has been
// applied via golang-migrate. Returns true only if schema_migrations exists
// and contains the given version.
func HasMigrationVersion(db *gorm.DB, version int) bool {
	if !TableExists(db, "schema_migrations") {
		return false
	}
	var count int64
	db.Raw("SELECT count(*) FROM schema_migrations WHERE version = ?", version).Scan(&count)
	return count > 0
}
