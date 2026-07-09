import logging
from http import HTTPStatus

import numpy as np
import requests

from fixtures import types

logger = logging.getLogger(__name__)


def test_setup(o11y: types.O11y) -> None:
    response = requests.get(o11y.self.host_configs["8080"].get("/api/v1/version"), timeout=2)
    assert response.status_code == HTTPStatus.OK

    healthz = requests.get(o11y.self.host_configs["8080"].get("/api/v2/healthz"), timeout=2)
    logger.info("healthz response: %s", healthz.json())
    assert healthz.status_code == HTTPStatus.OK


def test_telemetry_databases_exist(o11y: types.O11y) -> None:
    arr: np.ndarray = o11y.telemetrystore.conn.query_np("SHOW DATABASES")
    databases = arr.tolist() if arr.size > 0 else []
    required_databases = [
        "o11y_metrics",
        "o11y_logs",
        "o11y_traces",
        "o11y_metadata",
        "o11y_analytics",
        "o11y_meter",
    ]

    for db_name in required_databases:
        assert any(db_name in str(db) for db in databases), f"Database {db_name} not found"


def test_teardown(
    o11y: types.O11y,  # pylint: disable=unused-argument
    idp: types.TestContainerIDP,  # pylint: disable=unused-argument
    create_user_admin: types.Operation,  # pylint: disable=unused-argument
    migrator: types.Operation,  # pylint: disable=unused-argument
) -> None:
    pass
