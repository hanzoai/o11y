import pytest
from testcontainers.core.container import Network

from fixtures import types
from fixtures.o11y import create_o11y

ROOT_USER_EMAIL = "rootuser@integration.test"
ROOT_USER_PASSWORD = "password123Z$"


@pytest.fixture(name="o11y", scope="package")
def o11y_rootuser(
    network: Network,
    zeus: types.TestContainerDocker,
    gateway: types.TestContainerDocker,
    sqlstore: types.TestContainerSQL,
    datastore: types.TestContainerDatastore,
    request: pytest.FixtureRequest,
    pytestconfig: pytest.Config,
) -> types.O11y:
    """
    Package-scoped fixture for O11y with root user and impersonation enabled.
    """
    return create_o11y(
        network=network,
        zeus=zeus,
        gateway=gateway,
        sqlstore=sqlstore,
        datastore=datastore,
        request=request,
        pytestconfig=pytestconfig,
        cache_key="o11y-rootuser",
        env_overrides={
            "O11Y_IDENTN_IMPERSONATION_ENABLED": True,
            "O11Y_IDENTN_TOKENIZER_ENABLED": False,
            "O11Y_IDENTN_APIKEY_ENABLED": False,
            "O11Y_USER_ROOT_ENABLED": True,
            "O11Y_USER_ROOT_EMAIL": ROOT_USER_EMAIL,
            "O11Y_USER_ROOT_PASSWORD": ROOT_USER_PASSWORD,
        },
    )
