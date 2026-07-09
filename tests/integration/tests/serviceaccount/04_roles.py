from collections.abc import Callable
from http import HTTPStatus

import requests

from fixtures import types
from fixtures.auth import USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD
from fixtures.logger import setup_logger
from fixtures.serviceaccount import (
    SERVICE_ACCOUNT_BASE,
    create_service_account,
    create_service_account_with_key,
    find_role_by_name,
)

logger = setup_logger(__name__)


def test_get_service_account_roles(
    o11y: types.O11y,
    create_user_admin: types.Operation,  # pylint: disable=unused-argument
    get_token: Callable[[str, str], str],
):
    """GET /{id}/roles returns the assigned roles list."""
    token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)
    service_account_id = create_service_account(o11y, token, "sa-get-roles", role="o11y-viewer")

    response = requests.get(
        o11y.self.host_configs["8080"].get(f"{SERVICE_ACCOUNT_BASE}/{service_account_id}/roles"),
        headers={"Authorization": f"Bearer {token}"},
        timeout=5,
    )

    assert response.status_code == HTTPStatus.OK, response.text
    data = response.json()["data"]
    assert isinstance(data, list)
    assert len(data) >= 1
    role_names = [r["name"] for r in data]
    assert "o11y-viewer" in role_names


def test_assign_role_to_service_account(
    o11y: types.O11y,
    create_user_admin: types.Operation,  # pylint: disable=unused-argument
    get_token: Callable[[str, str], str],
):
    """POST /{id}/roles adds a role alongside existing ones."""
    token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)

    # create service account with viewer role
    service_account_id = create_service_account(o11y, token, "sa-assign-role", role="o11y-viewer")

    # assign editor role (additive — viewer stays)
    editor_role_id = find_role_by_name(o11y, token, "o11y-editor")
    assign_resp = requests.post(
        o11y.self.host_configs["8080"].get(f"{SERVICE_ACCOUNT_BASE}/{service_account_id}/roles"),
        json={"id": editor_role_id},
        headers={"Authorization": f"Bearer {token}"},
        timeout=5,
    )
    assert assign_resp.status_code == HTTPStatus.NO_CONTENT, assign_resp.text

    # verify both viewer and editor roles are present
    roles_resp = requests.get(
        o11y.self.host_configs["8080"].get(f"{SERVICE_ACCOUNT_BASE}/{service_account_id}/roles"),
        headers={"Authorization": f"Bearer {token}"},
        timeout=5,
    )
    assert roles_resp.status_code == HTTPStatus.OK, roles_resp.text
    role_names = [r["name"] for r in roles_resp.json()["data"]]
    assert len(role_names) == 2
    assert "o11y-viewer" in role_names
    assert "o11y-editor" in role_names

    # assign admin role — all three should be present
    admin_role_id = find_role_by_name(o11y, token, "o11y-admin")
    assign_resp = requests.post(
        o11y.self.host_configs["8080"].get(f"{SERVICE_ACCOUNT_BASE}/{service_account_id}/roles"),
        json={"id": admin_role_id},
        headers={"Authorization": f"Bearer {token}"},
        timeout=5,
    )
    assert assign_resp.status_code == HTTPStatus.NO_CONTENT, assign_resp.text

    roles_resp = requests.get(
        o11y.self.host_configs["8080"].get(f"{SERVICE_ACCOUNT_BASE}/{service_account_id}/roles"),
        headers={"Authorization": f"Bearer {token}"},
        timeout=5,
    )
    assert roles_resp.status_code == HTTPStatus.OK, roles_resp.text
    role_names = [r["name"] for r in roles_resp.json()["data"]]
    assert len(role_names) == 3
    assert "o11y-viewer" in role_names
    assert "o11y-editor" in role_names
    assert "o11y-admin" in role_names


def test_assign_role_idempotent(
    o11y: types.O11y,
    create_user_admin: types.Operation,  # pylint: disable=unused-argument
    get_token: Callable[[str, str], str],
):
    """POST same role twice succeeds (replace with same role is idempotent)."""
    token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)
    service_account_id = create_service_account(o11y, token, "sa-role-idempotent", role="o11y-viewer")

    viewer_role_id = find_role_by_name(o11y, token, "o11y-viewer")

    # assign the same role again
    resp = requests.post(
        o11y.self.host_configs["8080"].get(f"{SERVICE_ACCOUNT_BASE}/{service_account_id}/roles"),
        json={"id": viewer_role_id},
        headers={"Authorization": f"Bearer {token}"},
        timeout=5,
    )
    assert resp.status_code == HTTPStatus.NO_CONTENT, resp.text

    # verify only one instance of the role
    roles_resp = requests.get(
        o11y.self.host_configs["8080"].get(f"{SERVICE_ACCOUNT_BASE}/{service_account_id}/roles"),
        headers={"Authorization": f"Bearer {token}"},
        timeout=5,
    )
    role_names = [r["name"] for r in roles_resp.json()["data"]]
    assert role_names.count("o11y-viewer") == 1


def test_assign_role_expands_access(
    o11y: types.O11y,
    create_user_admin: types.Operation,  # pylint: disable=unused-argument
    get_token: Callable[[str, str], str],
):
    """Adding a higher-privilege role expands the SA's access."""
    token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)

    # create SA with viewer role and an API key
    service_account_id, api_key = create_service_account_with_key(o11y, token, "sa-role-expand-access", role="o11y-viewer")

    # viewer should get 403 on admin-only endpoint
    resp = requests.get(
        o11y.self.host_configs["8080"].get(SERVICE_ACCOUNT_BASE),
        headers={"O11Y-API-KEY": api_key},
        timeout=5,
    )
    assert resp.status_code == HTTPStatus.FORBIDDEN, f"Expected 403 for viewer on admin endpoint, got {resp.status_code}: {resp.text}"

    # assign admin role (additive — viewer stays)
    admin_role_id = find_role_by_name(o11y, token, "o11y-admin")
    assign_resp = requests.post(
        o11y.self.host_configs["8080"].get(f"{SERVICE_ACCOUNT_BASE}/{service_account_id}/roles"),
        json={"id": admin_role_id},
        headers={"Authorization": f"Bearer {token}"},
        timeout=5,
    )
    assert assign_resp.status_code == HTTPStatus.NO_CONTENT, assign_resp.text

    # SA should now have admin access
    resp = requests.get(
        o11y.self.host_configs["8080"].get(SERVICE_ACCOUNT_BASE),
        headers={"O11Y-API-KEY": api_key},
        timeout=5,
    )
    assert resp.status_code == HTTPStatus.OK, f"Expected 200 after adding admin role, got {resp.status_code}: {resp.text}"

    # verify both roles are present
    roles_resp = requests.get(
        o11y.self.host_configs["8080"].get(f"{SERVICE_ACCOUNT_BASE}/{service_account_id}/roles"),
        headers={"Authorization": f"Bearer {token}"},
        timeout=5,
    )
    assert roles_resp.status_code == HTTPStatus.OK, roles_resp.text
    role_names = [r["name"] for r in roles_resp.json()["data"]]
    assert len(role_names) == 2
    assert "o11y-admin" in role_names
    assert "o11y-viewer" in role_names


def test_remove_role_from_service_account(
    o11y: types.O11y,
    create_user_admin: types.Operation,  # pylint: disable=unused-argument
    get_token: Callable[[str, str], str],
):
    """DELETE /{id}/roles/{rid} revokes one role while keeping others."""
    token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)
    service_account_id = create_service_account(o11y, token, "sa-remove-role", role="o11y-editor")

    # add admin role (now has editor + admin)
    admin_role_id = find_role_by_name(o11y, token, "o11y-admin")
    assign_resp = requests.post(
        o11y.self.host_configs["8080"].get(f"{SERVICE_ACCOUNT_BASE}/{service_account_id}/roles"),
        json={"id": admin_role_id},
        headers={"Authorization": f"Bearer {token}"},
        timeout=5,
    )
    assert assign_resp.status_code == HTTPStatus.NO_CONTENT, assign_resp.text

    # remove editor role
    editor_role_id = find_role_by_name(o11y, token, "o11y-editor")
    resp = requests.delete(
        o11y.self.host_configs["8080"].get(f"{SERVICE_ACCOUNT_BASE}/{service_account_id}/roles/{editor_role_id}"),
        headers={"Authorization": f"Bearer {token}"},
        timeout=5,
    )
    assert resp.status_code == HTTPStatus.NO_CONTENT, resp.text

    # verify editor is gone but admin remains
    roles_resp = requests.get(
        o11y.self.host_configs["8080"].get(f"{SERVICE_ACCOUNT_BASE}/{service_account_id}/roles"),
        headers={"Authorization": f"Bearer {token}"},
        timeout=5,
    )
    assert roles_resp.status_code == HTTPStatus.OK, roles_resp.text
    role_names = [r["name"] for r in roles_resp.json()["data"]]
    assert "o11y-editor" not in role_names
    assert "o11y-admin" in role_names


def test_remove_role_verify_access_lost(
    o11y: types.O11y,
    create_user_admin: types.Operation,  # pylint: disable=unused-argument
    get_token: Callable[[str, str], str],
):
    """After role removal, service account key gets 403 on endpoints requiring that role."""
    token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)
    service_account_id, api_key = create_service_account_with_key(o11y, token, "sa-role-access-lost", role="o11y-admin")

    # verify admin access works
    resp = requests.get(
        o11y.self.host_configs["8080"].get(SERVICE_ACCOUNT_BASE),
        headers={"O11Y-API-KEY": api_key},
        timeout=5,
    )
    assert resp.status_code == HTTPStatus.OK, resp.text

    # remove admin role
    admin_role_id = find_role_by_name(o11y, token, "o11y-admin")
    del_resp = requests.delete(
        o11y.self.host_configs["8080"].get(f"{SERVICE_ACCOUNT_BASE}/{service_account_id}/roles/{admin_role_id}"),
        headers={"Authorization": f"Bearer {token}"},
        timeout=5,
    )
    assert del_resp.status_code == HTTPStatus.NO_CONTENT, del_resp.text

    # now admin endpoint should be forbidden
    resp = requests.get(
        o11y.self.host_configs["8080"].get(SERVICE_ACCOUNT_BASE),
        headers={"O11Y-API-KEY": api_key},
        timeout=5,
    )
    assert resp.status_code == HTTPStatus.FORBIDDEN, f"Expected 403 after role removal, got {resp.status_code}: {resp.text}"
