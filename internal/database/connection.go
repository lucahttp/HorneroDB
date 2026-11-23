package database

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/plugin/opentelemetry/tracing"
)

var DB *gorm.DB

type Config struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// isProduction reports whether the instance runs in production mode.
// Kept local to this package on purpose: the database layer must not depend
// on the configuration package (avoids an import cycle).
func isProduction() bool {
	return os.Getenv("HORNERO_ENV") == "production" || os.Getenv("ENV") == "production"
}

// quoteDSNValue escapes a value for a libpq keyword/value connection string.
// Values are single quoted and embedded quotes/backslashes are escaped, so
// passwords with spaces, quotes or backslashes cannot break the DSN (and
// cannot inject extra connection parameters).
func quoteDSNValue(value string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`'`, `\'`,
	)
	return "'" + replacer.Replace(value) + "'"
}

func Connect(cfg *Config) error {
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=UTC plan_cache_mode=force_custom_plan",
		quoteDSNValue(cfg.Host),
		quoteDSNValue(cfg.User),
		quoteDSNValue(cfg.Password),
		quoteDSNValue(cfg.DBName),
		quoteDSNValue(cfg.Port),
		quoteDSNValue(cfg.SSLMode),
	)

	// En producción no se loguea el SQL (ni los valores que viajan en él:
	// PII de los registros). Fuera de producción se mantiene el log detallado
	// para poder depurar.
	logLevel := logger.Info
	if isProduction() {
		logLevel = logger.Warn
	}

	var err error
	DB, err = gorm.Open(postgres.New(postgres.Config{
		DSN:                  dsn,
		PreferSimpleProtocol: true, // disables implicit prepared statement usage
	}), &gorm.Config{
		Logger:      logger.Default.LogMode(logLevel),
		PrepareStmt: false,
	})

	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	tracingOpts := []tracing.Option{
		tracing.WithDBSystem("postgresql"),
		tracing.WithoutMetrics(),
	}
	if isProduction() {
		// Never include query bind variables in production spans so row values / PII
		// cannot leak into telemetry storage.
		tracingOpts = append(tracingOpts, tracing.WithoutQueryVariables())
	}
	if err := DB.Use(tracing.NewPlugin(tracingOpts...)); err != nil {
		return fmt.Errorf("failed to register database tracing plugin: %w", err)
	}

	// Connection pool: evita agotar las conexiones de Postgres con trafico
	// concurrente y permite reciclar conexiones viejas.
	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)

	log.Println("✅ Connected to PostgreSQL")
	return nil
}

func Close() error {
	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
