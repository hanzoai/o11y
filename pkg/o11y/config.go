package o11y

import (
	"context"
	"log/slog"
	"os"
	"path"
	"reflect"
	"time"

	"github.com/hanzoai/o11y/pkg/alertmanager"
	"github.com/hanzoai/o11y/pkg/analytics"
	"github.com/hanzoai/o11y/pkg/apiserver"
	"github.com/hanzoai/o11y/pkg/auditor"
	"github.com/hanzoai/o11y/pkg/authz"
	"github.com/hanzoai/o11y/pkg/cache"
	"github.com/hanzoai/o11y/pkg/config"
	"github.com/hanzoai/o11y/pkg/emailing"
	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/flagger"
	"github.com/hanzoai/o11y/pkg/gateway"
	"github.com/hanzoai/o11y/pkg/global"
	"github.com/hanzoai/o11y/pkg/identn"
	"github.com/hanzoai/o11y/pkg/instrumentation"
	"github.com/hanzoai/o11y/pkg/meterreporter"
	"github.com/hanzoai/o11y/pkg/modules/cloudintegration"
	"github.com/hanzoai/o11y/pkg/modules/inframonitoring"
	"github.com/hanzoai/o11y/pkg/modules/metricsexplorer"
	"github.com/hanzoai/o11y/pkg/modules/serviceaccount"
	"github.com/hanzoai/o11y/pkg/modules/tracedetail"
	"github.com/hanzoai/o11y/pkg/modules/user"
	"github.com/hanzoai/o11y/pkg/pprof"
	"github.com/hanzoai/o11y/pkg/prometheus"
	"github.com/hanzoai/o11y/pkg/querier"
	"github.com/hanzoai/o11y/pkg/ruler"
	"github.com/hanzoai/o11y/pkg/sharder"
	"github.com/hanzoai/o11y/pkg/sqlmigration"
	"github.com/hanzoai/o11y/pkg/sqlmigrator"
	"github.com/hanzoai/o11y/pkg/sqlschema"
	"github.com/hanzoai/o11y/pkg/sqlstore"
	"github.com/hanzoai/o11y/pkg/statsreporter"
	"github.com/hanzoai/o11y/pkg/telemetrystore"
	"github.com/hanzoai/o11y/pkg/tokenizer"
	"github.com/hanzoai/o11y/pkg/valuer"
	"github.com/hanzoai/o11y/pkg/version"
	"github.com/hanzoai/o11y/pkg/web"
)

// Config defines the entire input configuration of o11y.
type Config struct {
	// Global config
	Global global.Config `mapstructure:"global"`

	// Version config
	Version version.Config `mapstructure:"version"`

	// Instrumentation config
	Instrumentation instrumentation.Config `mapstructure:"instrumentation"`

	// PProf config
	PProf pprof.Config `mapstructure:"pprof"`

	// Analytics config
	Analytics analytics.Config `mapstructure:"analytics"`

	// Web config
	Web web.Config `mapstructure:"web"`

	// Cache config
	Cache cache.Config `mapstructure:"cache"`

	// SQLStore config
	SQLStore sqlstore.Config `mapstructure:"sqlstore"`

	// SQLMigration config
	SQLMigration sqlmigration.Config `mapstructure:"sqlmigration"`

	// SQLMigrator config
	SQLMigrator sqlmigrator.Config `mapstructure:"sqlmigrator"`

	// SQLSchema config
	SQLSchema sqlschema.Config `mapstructure:"sqlschema"`

	// API Server config
	APIServer apiserver.Config `mapstructure:"apiserver"`

	// TelemetryStore config
	TelemetryStore telemetrystore.Config `mapstructure:"telemetrystore"`

	// Prometheus config
	Prometheus prometheus.Config `mapstructure:"prometheus"`

	// Alertmanager config
	Alertmanager alertmanager.Config `mapstructure:"alertmanager" yaml:"alertmanager"`

	// Querier config
	Querier querier.Config `mapstructure:"querier"`

	// Ruler config
	Ruler ruler.Config `mapstructure:"ruler"`

	// Emailing config
	Emailing emailing.Config `mapstructure:"emailing" yaml:"emailing"`

	// Sharder config
	Sharder sharder.Config `mapstructure:"sharder" yaml:"sharder"`

	// StatsReporter config
	StatsReporter statsreporter.Config `mapstructure:"statsreporter"`

	// Gateway config
	Gateway gateway.Config `mapstructure:"gateway"`

	// Tokenizer config
	Tokenizer tokenizer.Config `mapstructure:"tokenizer"`

	// MetricsExplorer config
	MetricsExplorer metricsexplorer.Config `mapstructure:"metricsexplorer"`

	// InfraMonitoring config
	InfraMonitoring inframonitoring.Config `mapstructure:"inframonitoring"`

	// Flagger config
	Flagger flagger.Config `mapstructure:"flagger"`

	// User config
	User user.Config `mapstructure:"user"`

	// IdentN config
	IdentN identn.Config `mapstructure:"identn"`

	// ServiceAccount config
	ServiceAccount serviceaccount.Config `mapstructure:"serviceaccount"`

	// Auditor config
	Auditor auditor.Config `mapstructure:"auditor"`

	// MeterReporter config
	MeterReporter meterreporter.Config `mapstructure:"meterreporter"`

	// CloudIntegration config
	CloudIntegration cloudintegration.Config `mapstructure:"cloudintegration"`

	// TraceDetail config
	TraceDetail tracedetail.Config `mapstructure:"traces"`

	// Authz config
	Authz authz.Config `mapstructure:"authz"`
}

func NewConfig(ctx context.Context, logger *slog.Logger, resolverConfig config.ResolverConfig) (Config, error) {
	configFactories := []factory.ConfigFactory{
		global.NewConfigFactory(),
		version.NewConfigFactory(),
		instrumentation.NewConfigFactory(),
		pprof.NewConfigFactory(),
		analytics.NewConfigFactory(),
		web.NewConfigFactory(),
		cache.NewConfigFactory(),
		sqlstore.NewConfigFactory(),
		sqlmigrator.NewConfigFactory(),
		sqlschema.NewConfigFactory(),
		apiserver.NewConfigFactory(),
		telemetrystore.NewConfigFactory(),
		prometheus.NewConfigFactory(),
		alertmanager.NewConfigFactory(),
		querier.NewConfigFactory(),
		ruler.NewConfigFactory(),
		emailing.NewConfigFactory(),
		sharder.NewConfigFactory(),
		statsreporter.NewConfigFactory(),
		gateway.NewConfigFactory(),
		tokenizer.NewConfigFactory(),
		metricsexplorer.NewConfigFactory(),
		inframonitoring.NewConfigFactory(),
		flagger.NewConfigFactory(),
		user.NewConfigFactory(),
		identn.NewConfigFactory(),
		serviceaccount.NewConfigFactory(),
		auditor.NewConfigFactory(),
		meterreporter.NewConfigFactory(),
		cloudintegration.NewConfigFactory(),
		tracedetail.NewConfigFactory(),
		authz.NewConfigFactory(),
	}

	conf, err := config.New(ctx, resolverConfig, configFactories)
	if err != nil {
		return Config{}, err
	}

	var config Config
	if err := conf.Unmarshal("", &config, "yaml"); err != nil {
		return Config{}, err
	}

	mergeAndEnsureBackwardCompatibility(ctx, logger, &config)

	if err := validateConfig(config); err != nil {
		return Config{}, err
	}

	return config, nil
}

func validateConfig(config Config) error {
	rvConfig := reflect.ValueOf(config)
	for i := 0; i < rvConfig.NumField(); i++ {
		factoryConfig, ok := rvConfig.Field(i).Interface().(factory.Config)
		if !ok {
			return errors.NewInvalidInputf(errors.CodeInvalidInput, "%q is not of type \"factory.Config\"", rvConfig.Type().Field(i).Name)
		}

		if err := factoryConfig.Validate(); err != nil {
			return errors.WrapInvalidInputf(err, errors.CodeInvalidInput, "failed to validate config %q", rvConfig.Type().Field(i).Tag.Get("mapstructure"))
		}
	}

	return nil
}

func mergeAndEnsureBackwardCompatibility(ctx context.Context, logger *slog.Logger, config *Config) {
	if os.Getenv("O11Y_LOCAL_DB_PATH") != "" {
		logger.WarnContext(ctx, "[Deprecated] env O11Y_LOCAL_DB_PATH is deprecated and scheduled for removal. Please use O11Y_SQLSTORE_SQLITE_PATH instead.")
		config.SQLStore.Sqlite.Path = os.Getenv("O11Y_LOCAL_DB_PATH")
	}

	if os.Getenv("CONTEXT_TIMEOUT") != "" {
		logger.WarnContext(ctx, "[Deprecated] env CONTEXT_TIMEOUT is deprecated and scheduled for removal. Please use O11Y_APISERVER_TIMEOUT_DEFAULT instead.")
		contextTimeoutDuration, err := time.ParseDuration(os.Getenv("CONTEXT_TIMEOUT") + "s")
		if err == nil {
			config.APIServer.Timeout.Default = contextTimeoutDuration
		} else {
			logger.WarnContext(ctx, "Error parsing CONTEXT_TIMEOUT, using default value of 60s")
		}
	}

	if os.Getenv("CONTEXT_TIMEOUT_MAX_ALLOWED") != "" {
		logger.WarnContext(ctx, "[Deprecated] env CONTEXT_TIMEOUT_MAX_ALLOWED is deprecated and scheduled for removal. Please use O11Y_APISERVER_TIMEOUT_MAX instead.")

		contextTimeoutDuration, err := time.ParseDuration(os.Getenv("CONTEXT_TIMEOUT_MAX_ALLOWED") + "s")
		if err == nil {
			config.APIServer.Timeout.Max = contextTimeoutDuration
		} else {
			logger.WarnContext(ctx, "Error parsing CONTEXT_TIMEOUT_MAX_ALLOWED, using default value of 600s")
		}
	}

	if os.Getenv("STORAGE") != "" {
		logger.WarnContext(ctx, "[Deprecated] env STORAGE is deprecated and scheduled for removal. Please use O11Y_TELEMETRYSTORE_PROVIDER instead.")
		config.TelemetryStore.Provider = os.Getenv("STORAGE")
	}

	if os.Getenv("DatastoreUrl") != "" {
		logger.WarnContext(ctx, "[Deprecated] env DatastoreUrl is deprecated and scheduled for removal. Please use O11Y_DATASTORE_DSN instead.")
		config.TelemetryStore.Datastore.DSN = os.Getenv("DatastoreUrl")
	}

	// O11Y_DATASTORE_DSN is the canonical, flat operator-facing key for the Hanzo
	// Datastore connection. It takes precedence over the structured
	// O11Y_TELEMETRYSTORE_DATASTORE_DSN (telemetrystore.datastore.dsn), which stays
	// as an internal fallback.
	if dsn := os.Getenv("O11Y_DATASTORE_DSN"); dsn != "" {
		config.TelemetryStore.Datastore.DSN = dsn
	}

	if os.Getenv("INVITE_EMAIL_TEMPLATE") != "" {
		logger.WarnContext(ctx, "[Deprecated] env INVITE_EMAIL_TEMPLATE is deprecated and scheduled for removal. Please use O11Y_EMAILING_TEMPLATES_DIRECTORY instead.")
		config.Emailing.Templates.Directory = path.Dir(os.Getenv("INVITE_EMAIL_TEMPLATE"))
	}

	if os.Getenv("SMTP_ENABLED") != "" {
		logger.WarnContext(ctx, "[Deprecated] env SMTP_ENABLED is deprecated and scheduled for removal. Please use O11Y_EMAILING_ENABLED instead.")
		config.Emailing.Enabled = os.Getenv("SMTP_ENABLED") == "true"
	}

	if os.Getenv("SMTP_HOST") != "" {
		logger.WarnContext(ctx, "[Deprecated] env SMTP_HOST is deprecated and scheduled for removal. Please use O11Y_EMAILING_ADDRESS instead.")
		if os.Getenv("SMTP_PORT") != "" {
			config.Emailing.SMTP.Address = os.Getenv("SMTP_HOST") + ":" + os.Getenv("SMTP_PORT")
		} else {
			config.Emailing.SMTP.Address = os.Getenv("SMTP_HOST")
		}
	}

	if os.Getenv("SMTP_PORT") != "" {
		logger.WarnContext(ctx, "[Deprecated] env SMTP_PORT is deprecated and scheduled for removal. Please use O11Y_EMAILING_ADDRESS instead.")
	}

	if os.Getenv("SMTP_USERNAME") != "" {
		logger.WarnContext(ctx, "[Deprecated] env SMTP_USERNAME is deprecated and scheduled for removal. Please use O11Y_EMAILING_AUTH_USERNAME instead.")
		config.Emailing.SMTP.Auth.Username = os.Getenv("SMTP_USERNAME")
	}

	if os.Getenv("SMTP_PASSWORD") != "" {
		logger.WarnContext(ctx, "[Deprecated] env SMTP_PASSWORD is deprecated and scheduled for removal. Please use O11Y_EMAILING_AUTH_PASSWORD instead.")
		config.Emailing.SMTP.Auth.Password = os.Getenv("SMTP_PASSWORD")
	}

	if os.Getenv("SMTP_FROM") != "" {
		logger.WarnContext(ctx, "[Deprecated] env SMTP_FROM is deprecated and scheduled for removal. Please use O11Y_EMAILING_FROM instead.")
		config.Emailing.SMTP.From = os.Getenv("SMTP_FROM")
	}

	if os.Getenv("O11Y_SAAS_SEGMENT_KEY") != "" {
		logger.WarnContext(ctx, "[Deprecated] env O11Y_SAAS_SEGMENT_KEY is deprecated and scheduled for removal. Please use O11Y_ANALYTICS_SEGMENT_KEY instead.")
		config.Analytics.Segment.Key = os.Getenv("O11Y_SAAS_SEGMENT_KEY")
	}

	if os.Getenv("TELEMETRY_ENABLED") != "" {
		logger.WarnContext(ctx, "[Deprecated] env TELEMETRY_ENABLED is deprecated and scheduled for removal. Please use O11Y_ANALYTICS_ENABLED instead.")
		config.Analytics.Enabled = os.Getenv("TELEMETRY_ENABLED") == "true"
	}

	if os.Getenv("USE_SPAN_METRICS") != "" {
		logger.WarnContext(ctx, "[Deprecated] env USE_SPAN_METRICS is deprecated and scheduled for removal. Please use O11Y_FLAGGER_CONFIG_BOOLEAN_USE__SPAN__METRICS instead.")
		if config.Flagger.Config.Boolean == nil {
			config.Flagger.Config.Boolean = make(map[string]bool)
		}
		config.Flagger.Config.Boolean[flagger.FeatureUseSpanMetrics.String()] = os.Getenv("USE_SPAN_METRICS") == "true"
	}

	if os.Getenv("O11Y_JWT_SECRET") != "" {
		logger.WarnContext(ctx, "[Deprecated] env O11Y_JWT_SECRET is deprecated and scheduled for removal. Please use O11Y_TOKENIZER_JWT_SECRET instead.")
		config.Tokenizer.JWT.Secret = os.Getenv("O11Y_JWT_SECRET")
	}

	if os.Getenv("KAFKA_SPAN_EVAL") != "" {
		logger.WarnContext(ctx, "[Deprecated] env KAFKA_SPAN_EVAL is deprecated and scheduled for removal. Please use O11Y_FLAGGER_CONFIG_BOOLEAN_KAFKA__SPAN__EVAL instead.")
		if config.Flagger.Config.Boolean == nil {
			config.Flagger.Config.Boolean = make(map[string]bool)
		}
		config.Flagger.Config.Boolean[flagger.FeatureKafkaSpanEval.String()] = os.Getenv("KAFKA_SPAN_EVAL") == "true"
	}

	if os.Getenv("RULES_EVAL_DELAY") != "" {
		logger.WarnContext(ctx, "[Deprecated] env RULES_EVAL_DELAY is deprecated and scheduled for removal. Please use O11Y_RULER_EVAL__DELAY instead.")
		if d, err := time.ParseDuration(os.Getenv("RULES_EVAL_DELAY")); err == nil {
			config.Ruler.EvalDelay = d
		} else {
			logger.WarnContext(ctx, "Error parsing RULES_EVAL_DELAY, using default value of 2m")
		}
	}
}

func (config Config) Collect(_ context.Context, _ valuer.UUID) (map[string]any, error) {
	stats := make(map[string]any)

	// SQL Store Config Stats
	stats["config.sqlstore.provider"] = config.SQLStore.Provider

	// Tokenizer Config Stats
	stats["config.tokenizer.provider"] = config.Tokenizer.Provider

	// Cache Config Stats
	stats["config.cache.provider"] = config.Cache.Provider

	return stats, nil
}
