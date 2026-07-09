from collections.abc import Callable
from http import HTTPStatus

import requests
from wiremock.resources.mappings import Mapping

from fixtures.auth import USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD, add_license
from fixtures.types import Operation, O11y, TestContainerDocker


def test_create_and_delete_dashboard_without_license(
    o11y: O11y,
    create_user_admin: Operation,  # pylint: disable=unused-argument
    get_token: Callable[[str, str], str],
):
    admin_token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)

    response = requests.post(
        o11y.self.host_configs["8080"].get("/api/v1/dashboards"),
        json={"title": "Sample Title", "uploadedGrafana": False, "version": "v5"},
        headers={"Authorization": f"Bearer {admin_token}"},
        timeout=2,
    )

    assert response.status_code == HTTPStatus.CREATED
    assert response.json()["status"] == "success"
    data = response.json()["data"]
    dashboard_id = data["id"]

    response = requests.delete(
        o11y.self.host_configs["8080"].get(f"/api/v1/dashboards/{dashboard_id}"),
        headers={"Authorization": f"Bearer {admin_token}"},
        timeout=2,
    )

    assert response.status_code == HTTPStatus.NO_CONTENT


def test_apply_license(
    o11y: O11y,
    create_user_admin: Operation,  # pylint: disable=unused-argument
    make_http_mocks: Callable[[TestContainerDocker, list[Mapping]], None],
    get_token: Callable[[str, str], str],
) -> None:
    """
    This applies a license to the o11y instance.
    """
    add_license(o11y, make_http_mocks, get_token)


def test_create_and_delete_dashboard_with_license(
    o11y: O11y,
    create_user_admin: Operation,  # pylint: disable=unused-argument
    get_token: Callable[[str, str], str],
):
    admin_token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)

    response = requests.post(
        o11y.self.host_configs["8080"].get("/api/v1/dashboards"),
        json={"title": "Sample Title", "uploadedGrafana": False, "version": "v5"},
        headers={"Authorization": f"Bearer {admin_token}"},
        timeout=2,
    )

    assert response.status_code == HTTPStatus.CREATED
    assert response.json()["status"] == "success"
    data = response.json()["data"]
    dashboard_id = data["id"]

    response = requests.delete(
        o11y.self.host_configs["8080"].get(f"/api/v1/dashboards/{dashboard_id}"),
        headers={"Authorization": f"Bearer {admin_token}"},
        timeout=2,
    )

    assert response.status_code == HTTPStatus.NO_CONTENT
