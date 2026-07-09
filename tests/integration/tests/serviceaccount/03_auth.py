from collections.abc import Callable
from http import HTTPStatus

import requests

from fixtures import types
from fixtures.auth import USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD
from fixtures.logger import setup_logger
from fixtures.serviceaccount import (
    SERVICE_ACCOUNT_BASE,
    create_service_account_with_key,
    delete_service_account,
)

logger = setup_logger(__name__)


def test_service_account_key_auth_on_dashboards(
    o11y: types.O11y,
    create_user_admin: types.Operation,  # pylint: disable=unused-argument
    get_token: Callable[[str, str], str],
):
    """Service account API key with admin role can access dashboards."""
    token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)
    _, api_key = create_service_account_with_key(o11y, token, "sa-dashboard-test")

    response = requests.get(
        o11y.self.host_configs["8080"].get("/api/v1/dashboards"),
        headers={"O11Y-API-KEY": api_key},
        timeout=5,
    )

    assert response.status_code == HTTPStatus.OK, response.text


def test_service_account_key_forbidden_on_user_me(
    o11y: types.O11y,
    create_user_admin: types.Operation,  # pylint: disable=unused-argument
    get_token: Callable[[str, str], str],
):
    """Service account key must not access /api/v2/users/me — it's user-only."""
    token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)
    _, api_key = create_service_account_with_key(o11y, token, "sa-user-me-test")

    response = requests.get(
        o11y.self.host_configs["8080"].get("/api/v2/users/me"),
        headers={"O11Y-API-KEY": api_key},
        timeout=5,
    )

    ## This shouldn't be allowed on api key identn, will be updated once we fix that.
    assert response.status_code == HTTPStatus.NOT_FOUND, f"Expected 404 for service account on /users/me, got {response.status_code}: {response.text}"


def test_service_account_key_forbidden_on_user_preferences(
    o11y: types.O11y,
    create_user_admin: types.Operation,  # pylint: disable=unused-argument
    get_token: Callable[[str, str], str],
):
    """Service account key must not access user preference endpoints."""
    token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)
    _, api_key = create_service_account_with_key(o11y, token, "sa-pref-test")

    response = requests.get(
        o11y.self.host_configs["8080"].get("/api/v1/user/preferences"),
        headers={"O11Y-API-KEY": api_key},
        timeout=5,
    )

    ## This shouldn't be allowed on api key identn, will be updated once we fix that.
    assert response.status_code == HTTPStatus.OK, f"Expected 200 for service account on /user/preferences, got {response.status_code}: {response.text}"


def test_service_account_role_access_admin(
    o11y: types.O11y,
    create_user_admin: types.Operation,  # pylint: disable=unused-argument
    get_token: Callable[[str, str], str],
):
    """Admin service account can access admin, edit, and view endpoints."""
    token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)
    _, api_key = create_service_account_with_key(o11y, token, "sa-role-admin", role="o11y-admin")

    # AdminAccess: list service accounts
    resp = requests.get(
        o11y.self.host_configs["8080"].get(SERVICE_ACCOUNT_BASE),
        headers={"O11Y-API-KEY": api_key},
        timeout=5,
    )
    assert resp.status_code == HTTPStatus.OK, f"Admin service account should access admin endpoint, got {resp.status_code}: {resp.text}"

    # EditAccess: create a dashboard
    resp = requests.post(
        o11y.self.host_configs["8080"].get("/api/v1/dashboards"),
        json={
            "title": "admin-sa-dash",
            "uploadedGrafana": False,
            "version": "v4",
        },
        headers={"O11Y-API-KEY": api_key},
        timeout=5,
    )
    assert resp.status_code == HTTPStatus.CREATED, f"Admin service account should access edit endpoint, got {resp.status_code}: {resp.text}"

    # ViewAccess: list dashboards
    resp = requests.get(
        o11y.self.host_configs["8080"].get("/api/v1/dashboards"),
        headers={"O11Y-API-KEY": api_key},
        timeout=5,
    )
    assert resp.status_code == HTTPStatus.OK, f"Admin service account should access view endpoint, got {resp.status_code}: {resp.text}"


def test_service_account_role_access_editor(
    o11y: types.O11y,
    create_user_admin: types.Operation,  # pylint: disable=unused-argument
    get_token: Callable[[str, str], str],
):
    """Editor service account can access edit and view endpoints but not admin."""
    token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)
    _, api_key = create_service_account_with_key(o11y, token, "sa-role-editor", role="o11y-editor")

    # AdminAccess: should be forbidden
    resp = requests.get(
        o11y.self.host_configs["8080"].get(SERVICE_ACCOUNT_BASE),
        headers={"O11Y-API-KEY": api_key},
        timeout=5,
    )
    assert resp.status_code == HTTPStatus.FORBIDDEN, f"Editor service account should be forbidden from admin endpoint, got {resp.status_code}: {resp.text}"

    # EditAccess: create a dashboard
    resp = requests.post(
        o11y.self.host_configs["8080"].get("/api/v1/dashboards"),
        json={
            "title": "editor-sa-dash",
            "uploadedGrafana": False,
            "version": "v4",
        },
        headers={"O11Y-API-KEY": api_key},
        timeout=5,
    )
    assert resp.status_code == HTTPStatus.CREATED, f"Editor service account should access edit endpoint, got {resp.status_code}: {resp.text}"

    # ViewAccess: list dashboards
    resp = requests.get(
        o11y.self.host_configs["8080"].get("/api/v1/dashboards"),
        headers={"O11Y-API-KEY": api_key},
        timeout=5,
    )
    assert resp.status_code == HTTPStatus.OK, f"Editor service account should access view endpoint, got {resp.status_code}: {resp.text}"


def test_service_account_role_access_viewer(
    o11y: types.O11y,
    create_user_admin: types.Operation,  # pylint: disable=unused-argument
    get_token: Callable[[str, str], str],
):
    """Viewer service account can access view endpoints but not edit or admin."""
    token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)
    _, api_key = create_service_account_with_key(o11y, token, "sa-role-viewer", role="o11y-viewer")

    # AdminAccess: should be forbidden
    resp = requests.get(
        o11y.self.host_configs["8080"].get(SERVICE_ACCOUNT_BASE),
        headers={"O11Y-API-KEY": api_key},
        timeout=5,
    )
    assert resp.status_code == HTTPStatus.FORBIDDEN, f"Viewer service account should be forbidden from admin endpoint, got {resp.status_code}: {resp.text}"

    # EditAccess: should be forbidden
    resp = requests.post(
        o11y.self.host_configs["8080"].get("/api/v1/dashboards"),
        json={
            "title": "viewer-sa-dash",
            "uploadedGrafana": False,
            "version": "v4",
        },
        headers={"O11Y-API-KEY": api_key},
        timeout=5,
    )
    assert resp.status_code == HTTPStatus.FORBIDDEN, f"Viewer service account should be forbidden from edit endpoint, got {resp.status_code}: {resp.text}"

    # ViewAccess: list dashboards
    resp = requests.get(
        o11y.self.host_configs["8080"].get("/api/v1/dashboards"),
        headers={"O11Y-API-KEY": api_key},
        timeout=5,
    )
    assert resp.status_code == HTTPStatus.OK, f"Viewer service account should access view endpoint, got {resp.status_code}: {resp.text}"


def test_service_account_key_deleted_account_rejected(
    o11y: types.O11y,
    create_user_admin: types.Operation,  # pylint: disable=unused-argument
    get_token: Callable[[str, str], str],
):
    """A deleted service account's key must be rejected."""
    token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)
    service_account_id, api_key = create_service_account_with_key(o11y, token, "sa-disable-auth")

    # verify the key works before deleting
    response = requests.get(
        o11y.self.host_configs["8080"].get("/api/v1/dashboards"),
        headers={"O11Y-API-KEY": api_key},
        timeout=5,
    )
    assert response.status_code == HTTPStatus.OK

    # soft-delete the SA
    delete_service_account(o11y, token, service_account_id)

    # now the key should be rejected
    response = requests.get(
        o11y.self.host_configs["8080"].get("/api/v1/dashboards"),
        headers={"O11Y-API-KEY": api_key},
        timeout=5,
    )

    assert response.status_code == HTTPStatus.UNAUTHORIZED, f"Expected 401 for disabled service account, got {response.status_code}: {response.text}"


def test_service_account_key_revoked_key_rejected(
    o11y: types.O11y,
    create_user_admin: types.Operation,  # pylint: disable=unused-argument
    get_token: Callable[[str, str], str],
):
    """A revoked API key must be rejected."""
    token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)
    service_account_id, api_key = create_service_account_with_key(o11y, token, "sa-revoke-auth")

    # verify the key works first
    response = requests.get(
        o11y.self.host_configs["8080"].get("/api/v1/dashboards"),
        headers={"O11Y-API-KEY": api_key},
        timeout=5,
    )
    assert response.status_code == HTTPStatus.OK

    # find the key id
    keys_resp = requests.get(
        o11y.self.host_configs["8080"].get(f"{SERVICE_ACCOUNT_BASE}/{service_account_id}/keys"),
        headers={"Authorization": f"Bearer {token}"},
        timeout=5,
    )
    key_id = keys_resp.json()["data"][0]["id"]

    # revoke it
    revoke_resp = requests.delete(
        o11y.self.host_configs["8080"].get(f"{SERVICE_ACCOUNT_BASE}/{service_account_id}/keys/{key_id}"),
        headers={"Authorization": f"Bearer {token}"},
        timeout=5,
    )
    assert revoke_resp.status_code == HTTPStatus.NO_CONTENT

    # now the key should be rejected
    response = requests.get(
        o11y.self.host_configs["8080"].get("/api/v1/dashboards"),
        headers={"O11Y-API-KEY": api_key},
        timeout=5,
    )

    assert response.status_code == HTTPStatus.UNAUTHORIZED, f"Expected 401 for revoked key, got {response.status_code}: {response.text}"


def test_user_token_still_works_on_user_me(
    o11y: types.O11y,
    create_user_admin: types.Operation,  # pylint: disable=unused-argument
    get_token: Callable[[str, str], str],
):
    """Verify that normal user JWT tokens still work on user-only endpoints."""
    token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)

    response = requests.get(
        o11y.self.host_configs["8080"].get("/api/v2/users/me"),
        headers={"Authorization": f"Bearer {token}"},
        timeout=5,
    )

    assert response.status_code == HTTPStatus.OK, response.text
    data = response.json()["data"]
    assert data["email"] == USER_ADMIN_EMAIL


def test_user_token_still_works_on_user_preferences(
    o11y: types.O11y,
    create_user_admin: types.Operation,  # pylint: disable=unused-argument
    get_token: Callable[[str, str], str],
):
    """Verify that normal user JWT tokens still work on preference endpoints."""
    token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)

    response = requests.get(
        o11y.self.host_configs["8080"].get("/api/v1/user/preferences"),
        headers={"Authorization": f"Bearer {token}"},
        timeout=5,
    )

    assert response.status_code == HTTPStatus.OK, response.text
    assert response.json()["data"] is not None
