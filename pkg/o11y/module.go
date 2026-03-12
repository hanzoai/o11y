package o11y

import (
	"github.com/hanzoai/o11y/pkg/alertmanager"
	"github.com/hanzoai/o11y/pkg/analytics"
	"github.com/hanzoai/o11y/pkg/authn"
	"github.com/hanzoai/o11y/pkg/authz"
	"github.com/hanzoai/o11y/pkg/cache"
	"github.com/hanzoai/o11y/pkg/emailing"
	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/modules/apdex"
	"github.com/hanzoai/o11y/pkg/modules/apdex/implapdex"
	"github.com/hanzoai/o11y/pkg/modules/authdomain"
	"github.com/hanzoai/o11y/pkg/modules/authdomain/implauthdomain"
	"github.com/hanzoai/o11y/pkg/modules/dashboard"
	"github.com/hanzoai/o11y/pkg/modules/metricsexplorer"
	"github.com/hanzoai/o11y/pkg/modules/metricsexplorer/implmetricsexplorer"
	"github.com/hanzoai/o11y/pkg/modules/organization"
	"github.com/hanzoai/o11y/pkg/modules/organization/implorganization"
	"github.com/hanzoai/o11y/pkg/modules/preference"
	"github.com/hanzoai/o11y/pkg/modules/preference/implpreference"
	"github.com/hanzoai/o11y/pkg/modules/promote"
	"github.com/hanzoai/o11y/pkg/modules/promote/implpromote"
	"github.com/hanzoai/o11y/pkg/modules/quickfilter"
	"github.com/hanzoai/o11y/pkg/modules/quickfilter/implquickfilter"
	"github.com/hanzoai/o11y/pkg/modules/rawdataexport"
	"github.com/hanzoai/o11y/pkg/modules/rawdataexport/implrawdataexport"
	"github.com/hanzoai/o11y/pkg/modules/savedview"
	"github.com/hanzoai/o11y/pkg/modules/savedview/implsavedview"
	"github.com/hanzoai/o11y/pkg/modules/serviceaccount"
	"github.com/hanzoai/o11y/pkg/modules/serviceaccount/implserviceaccount"
	"github.com/hanzoai/o11y/pkg/modules/services"
	"github.com/hanzoai/o11y/pkg/modules/services/implservices"
	"github.com/hanzoai/o11y/pkg/modules/session"
	"github.com/hanzoai/o11y/pkg/modules/session/implsession"
	"github.com/hanzoai/o11y/pkg/modules/spanpercentile"
	"github.com/hanzoai/o11y/pkg/modules/spanpercentile/implspanpercentile"
	"github.com/hanzoai/o11y/pkg/modules/tracefunnel"
	"github.com/hanzoai/o11y/pkg/modules/tracefunnel/impltracefunnel"
	"github.com/hanzoai/o11y/pkg/modules/user"
	"github.com/hanzoai/o11y/pkg/modules/user/impluser"
	"github.com/hanzoai/o11y/pkg/querier"
	"github.com/hanzoai/o11y/pkg/queryparser"
	"github.com/hanzoai/o11y/pkg/ruler/rulestore/sqlrulestore"
	"github.com/hanzoai/o11y/pkg/sqlstore"
	"github.com/hanzoai/o11y/pkg/telemetrystore"
	"github.com/hanzoai/o11y/pkg/tokenizer"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/hanzoai/o11y/pkg/types/preferencetypes"
	"github.com/hanzoai/o11y/pkg/types/telemetrytypes"
)

type Modules struct {
	OrgGetter       organization.Getter
	OrgSetter       organization.Setter
	Preference      preference.Module
	User            user.Module
	UserGetter      user.Getter
	SavedView       savedview.Module
	Apdex           apdex.Module
	Dashboard       dashboard.Module
	QuickFilter     quickfilter.Module
	TraceFunnel     tracefunnel.Module
	RawDataExport   rawdataexport.Module
	AuthDomain      authdomain.Module
	Session         session.Module
	Services        services.Module
	SpanPercentile  spanpercentile.Module
	MetricsExplorer metricsexplorer.Module
	Promote         promote.Module
	ServiceAccount  serviceaccount.Module
}

func NewModules(
	sqlstore sqlstore.SQLStore,
	tokenizer tokenizer.Tokenizer,
	emailing emailing.Emailing,
	providerSettings factory.ProviderSettings,
	orgGetter organization.Getter,
	alertmanager alertmanager.Alertmanager,
	analytics analytics.Analytics,
	querier querier.Querier,
	telemetryStore telemetrystore.TelemetryStore,
	telemetryMetadataStore telemetrytypes.MetadataStore,
	authNs map[authtypes.AuthNProvider]authn.AuthN,
	authz authz.AuthZ,
	cache cache.Cache,
	queryParser queryparser.QueryParser,
	config Config,
	dashboard dashboard.Module,
	userGetter user.Getter,
) Modules {
	quickfilter := implquickfilter.NewModule(implquickfilter.NewStore(sqlstore))
	orgSetter := implorganization.NewSetter(implorganization.NewStore(sqlstore), alertmanager, quickfilter)
	user := impluser.NewModule(impluser.NewStore(sqlstore, providerSettings), tokenizer, emailing, providerSettings, orgSetter, authz, analytics, config.User)
	ruleStore := sqlrulestore.NewRuleStore(sqlstore, queryParser, providerSettings)

	return Modules{
		OrgGetter:       orgGetter,
		OrgSetter:       orgSetter,
		Preference:      implpreference.NewModule(implpreference.NewStore(sqlstore), preferencetypes.NewAvailablePreference()),
		SavedView:       implsavedview.NewModule(sqlstore),
		Apdex:           implapdex.NewModule(sqlstore),
		Dashboard:       dashboard,
		User:            user,
		UserGetter:      userGetter,
		QuickFilter:     quickfilter,
		TraceFunnel:     impltracefunnel.NewModule(impltracefunnel.NewStore(sqlstore)),
		RawDataExport:   implrawdataexport.NewModule(querier),
		AuthDomain:      implauthdomain.NewModule(implauthdomain.NewStore(sqlstore), authNs),
		Session:         implsession.NewModule(providerSettings, authNs, user, userGetter, implauthdomain.NewModule(implauthdomain.NewStore(sqlstore), authNs), tokenizer, orgGetter),
		SpanPercentile:  implspanpercentile.NewModule(querier, providerSettings),
		Services:        implservices.NewModule(querier, telemetryStore),
		MetricsExplorer: implmetricsexplorer.NewModule(telemetryStore, telemetryMetadataStore, cache, ruleStore, dashboard, providerSettings, config.MetricsExplorer),
		Promote:         implpromote.NewModule(telemetryMetadataStore, telemetryStore),
		ServiceAccount:  implserviceaccount.NewModule(implserviceaccount.NewStore(sqlstore), authz, emailing, providerSettings),
	}
}
