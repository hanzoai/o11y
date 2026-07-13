import pytest
from testcontainers.core.container import Network

from fixtures import types
from fixtures.migrator import create_migrator
from fixtures.o11y import create_o11y

UNSUPPORTED_CLICKHOUSE_VERSIONS = {"25.5.6"}


def pytest_collection_modifyitems(config: pytest.Config, items: list[pytest.Item]) -> None:
    version = config.getoption("--datastore-version")
    if version in UNSUPPORTED_CLICKHOUSE_VERSIONS:
        skip = pytest.mark.skip(reason=f"JSON body QB tests require ClickHouse > {version}")
        for item in items:
            item.add_marker(skip)


@pytest.fixture(name="migrator", scope="package")
def migrator_json(
    network: Network,
    datastore: types.TestContainerDatastore,
    request: pytest.FixtureRequest,
    pytestconfig: pytest.Config,
) -> types.Operation:
    """
    Package-scoped migrator with ENABLE_LOGS_MIGRATIONS_V2=1.
    """
    return create_migrator(
        network=network,
        datastore=datastore,
        request=request,
        pytestconfig=pytestconfig,
        cache_key="migrator-json-body",
        env_overrides={
            "ENABLE_LOGS_MIGRATIONS_V2": "1",
        },
    )


@pytest.fixture(name="o11y", scope="package")
def o11y_json_body(
    network: Network,
    migrator: types.Operation,  # pylint: disable=unused-argument
    zeus: types.TestContainerDocker,
    gateway: types.TestContainerDocker,
    sqlstore: types.TestContainerSQL,
    datastore: types.TestContainerDatastore,
    request: pytest.FixtureRequest,
    pytestconfig: pytest.Config,
) -> types.O11y:
    """
    Package-scoped fixture for O11y with BODY_JSON_QUERY_ENABLED=true.
    """
    return create_o11y(
        network=network,
        zeus=zeus,
        gateway=gateway,
        sqlstore=sqlstore,
        datastore=datastore,
        request=request,
        pytestconfig=pytestconfig,
        cache_key="o11y-json-body",
        env_overrides={
            "O11Y_FLAGGER_CONFIG_BOOLEAN_USE__JSON__BODY": True,
        },
    )
