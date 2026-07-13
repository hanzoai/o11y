import platform
import time
from http import HTTPStatus
from os import path

import docker
import docker.errors
import pytest
import requests
from testcontainers.core.container import DockerContainer, Network
from testcontainers.core.image import DockerImage

from fixtures import reuse, types
from fixtures.logger import setup_logger

logger = setup_logger(__name__)


def create_o11y(
    network: Network,
    zeus: types.TestContainerDocker,
    gateway: types.TestContainerDocker,
    sqlstore: types.TestContainerSQL,
    datastore: types.TestContainerDatastore,
    request: pytest.FixtureRequest,
    pytestconfig: pytest.Config,
    cache_key: str = "o11y",
    env_overrides: dict | None = None,
) -> types.O11y:
    """
    Factory function for creating a O11y container.
    Accepts optional env_overrides to customize the container environment.
    """

    def create() -> types.O11y:
        # Run the migrations for clickhouse
        request.getfixturevalue("migrator")

        # Get the no-web flag
        with_web = pytestconfig.getoption("--with-web")

        arch = platform.machine()
        if arch == "x86_64":
            arch = "amd64"

        # Build the image
        dockerfile_path = "cmd/enterprise/Dockerfile.integration"
        if with_web:
            dockerfile_path = "cmd/enterprise/Dockerfile.with-web.integration"

        # Docker build context is the repo root — one up from pytest's
        # rootdir (tests/).
        self = DockerImage(
            path=str(pytestconfig.rootpath.parent),
            dockerfile_path=dockerfile_path,
            tag="o11y:integration",
            buildargs={
                "TARGETARCH": arch,
                "ZEUSURL": zeus.container_configs["8080"].base(),
            },
        )

        self.build()

        env = (
            {
                "O11Y_WEB_ENABLED": False,
                "O11Y_WEB_DIRECTORY": "/root/web",
                "O11Y_INSTRUMENTATION_LOGS_LEVEL": "debug",
                "O11Y_PROMETHEUS_ACTIVE__QUERY__TRACKER_ENABLED": False,
                "O11Y_GATEWAY_URL": gateway.container_configs["8080"].base(),
                "O11Y_TOKENIZER_JWT_SECRET": "secret",
                "O11Y_GLOBAL_INGESTION__URL": "https://ingest.test.o11y.cloud",
                "O11Y_USER_PASSWORD_RESET_ALLOW__SELF": True,
                "O11Y_USER_PASSWORD_RESET_MAX__TOKEN__LIFETIME": "6h",
                "RULES_EVAL_DELAY": "0s",
                "O11Y_ALERTMANAGER_O11Y_POLL__INTERVAL": "5s",
                "O11Y_ALERTMANAGER_O11Y_ROUTE_GROUP__WAIT": "1s",
                "O11Y_ALERTMANAGER_O11Y_ROUTE_GROUP__INTERVAL": "5s",
                "O11Y_CLOUDINTEGRATION_AGENT_VERSION": "v0.0.8",
            }
            | sqlstore.env
            | datastore.env
        )

        if with_web:
            env["O11Y_WEB_ENABLED"] = True

        if env_overrides:
            env = env | env_overrides

        container = DockerContainer("o11y:integration")
        for k, v in env.items():
            container.with_env(k, v)
        container.with_exposed_ports(8080)
        container.with_network(network=network)

        provider = request.config.getoption("--sqlstore-provider")
        if provider == "sqlite":
            dir_path = path.dirname(sqlstore.env["O11Y_SQLSTORE_SQLITE_PATH"])
            container.with_volume_mapping(
                dir_path,
                dir_path,
                "rw",
            )

        container.start()

        def ready(container: DockerContainer) -> None:
            for attempt in range(10):
                try:
                    response = requests.get(
                        f"http://{container.get_container_host_ip()}:{container.get_exposed_port(8080)}/api/v2/healthz",
                        timeout=2,
                    )
                    if response.status_code == HTTPStatus.OK:
                        return
                    if response.status_code == HTTPStatus.SERVICE_UNAVAILABLE:
                        logger.error(
                            "Attempt %s: O11y container %s not ready yet:\n%s",
                            attempt + 1,
                            container,
                            response.text,
                        )
                except Exception as e:  # pylint: disable=broad-exception-caught
                    logger.error(
                        "Attempt %s at readiness check for O11y container %s failed: %s",
                        attempt + 1,
                        container,
                        e,
                    )
                time.sleep(2)
            raise TimeoutError("timeout exceeded while waiting")

        try:
            ready(container=container)
        except Exception as e:  # pylint: disable=broad-exception-caught
            raise e

        return types.O11y(
            self=types.TestContainerDocker(
                id=container.get_wrapped_container().id,
                host_configs={
                    "8080": types.TestContainerUrlConfig(
                        "http",
                        container.get_container_host_ip(),
                        container.get_exposed_port(8080),
                    )
                },
                container_configs={
                    "8080": types.TestContainerUrlConfig(
                        "http",
                        container.get_wrapped_container().name,
                        8080,
                    )
                },
            ),
            sqlstore=sqlstore,
            telemetrystore=datastore,
            zeus=zeus,
            gateway=gateway,
        )

    def delete(container: types.O11y) -> None:
        client = docker.from_env()
        try:
            client.containers.get(container_id=container.self.id).stop()
            client.containers.get(container_id=container.self.id).remove(v=True)
        except docker.errors.NotFound:
            logger.info(
                "Skipping removal of O11y, O11y(%s) not found. Maybe it was manually removed?",
                {"id": container.self.id},
            )

    def restore(cache: dict) -> types.O11y:
        self = types.TestContainerDocker.from_cache(cache)
        return types.O11y(
            self=self,
            sqlstore=sqlstore,
            telemetrystore=datastore,
            zeus=zeus,
            gateway=gateway,
        )

    return reuse.wrap(
        request,
        pytestconfig,
        cache_key,
        empty=lambda: types.O11y(
            self=types.TestContainerDocker(
                id="",
                host_configs={},
                container_configs={},
            ),
            sqlstore=sqlstore,
            telemetrystore=datastore,
            zeus=zeus,
            gateway=gateway,
        ),
        create=create,
        delete=delete,
        restore=restore,
    )


@pytest.fixture(name="o11y", scope="package")
def o11y(  # pylint: disable=too-many-arguments,too-many-positional-arguments
    network: Network,
    zeus: types.TestContainerDocker,
    gateway: types.TestContainerDocker,
    sqlstore: types.TestContainerSQL,
    datastore: types.TestContainerDatastore,
    request: pytest.FixtureRequest,
    pytestconfig: pytest.Config,
) -> types.O11y:
    """
    Package-scoped fixture for setting up O11y.
    """
    return create_o11y(
        network=network,
        zeus=zeus,
        gateway=gateway,
        sqlstore=sqlstore,
        datastore=datastore,
        request=request,
        pytestconfig=pytestconfig,
    )
