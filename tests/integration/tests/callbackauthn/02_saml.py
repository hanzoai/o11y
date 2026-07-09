import uuid
from collections.abc import Callable
from http import HTTPStatus
from typing import Any

import requests
from selenium import webdriver
from sqlalchemy import sql
from wiremock.resources.mappings import Mapping

from fixtures.auth import (
    USER_ADMIN_EMAIL,
    USER_ADMIN_PASSWORD,
    add_license,
    assert_user_has_role,
    find_user_with_roles_by_email,
)
from fixtures.idp import (
    get_saml_domain,
    perform_saml_login,
)
from fixtures.types import Operation, O11y, TestContainerDocker, TestContainerIDP


def test_apply_license(
    o11y: O11y,
    create_user_admin: Operation,  # pylint: disable=unused-argument
    make_http_mocks: Callable[[TestContainerDocker, list[Mapping]], None],
    get_token: Callable[[str, str], str],
) -> None:
    add_license(o11y, make_http_mocks, get_token)


def test_create_auth_domain(
    o11y: O11y,
    idp: TestContainerIDP,  # pylint: disable=unused-argument
    create_saml_client: Callable[[str, str], None],
    update_saml_client_attributes: Callable[[str, dict[str, Any]], None],
    get_saml_settings: Callable[[], dict],
    create_user_admin: Callable[[], None],  # pylint: disable=unused-argument
    get_token: Callable[[str, str], str],
) -> None:
    # Create a saml client in the idp.
    create_saml_client("saml.integration.test", "/api/v1/complete/saml")

    # Get the saml settings from keycloak.
    settings = get_saml_settings()

    # Create a auth domain in o11y.
    admin_token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)

    response = requests.post(
        o11y.self.host_configs["8080"].get("/api/v1/domains"),
        json={
            "name": "saml.integration.test",
            "config": {
                "ssoEnabled": True,
                "ssoType": "saml",
                "samlConfig": {
                    "samlEntity": settings["entityID"],
                    "samlIdp": settings["singleSignOnServiceLocation"],
                    "samlCert": settings["certificate"],
                },
            },
        },
        headers={"Authorization": f"Bearer {admin_token}"},
        timeout=2,
    )

    assert response.status_code == HTTPStatus.CREATED

    # Get the domains from o11y
    response = requests.get(
        o11y.self.host_configs["8080"].get("/api/v1/domains"),
        headers={"Authorization": f"Bearer {admin_token}"},
        timeout=2,
    )

    assert response.status_code == HTTPStatus.OK

    found_domain = None

    if len(response.json()["data"]) > 0:
        found_domain = next(
            (domain for domain in response.json()["data"] if domain["name"] == "saml.integration.test"),
            None,
        )

    relay_state_path = found_domain["authNProviderInfo"]["relayStatePath"]

    assert relay_state_path is not None

    # Get the relay state url from domains API
    relay_state_url = o11y.self.host_configs["8080"].base() + "/" + relay_state_path

    # Update the saml client with new attributes
    update_saml_client_attributes(
        f"{o11y.self.host_configs['8080'].address}:{o11y.self.host_configs['8080'].port}",
        {
            "saml_idp_initiated_sso_url_name": "idp-initiated-saml-test",
            "saml_idp_initiated_sso_relay_state": relay_state_url,
        },
    )


def test_saml_authn(
    o11y: O11y,
    idp: TestContainerIDP,  # pylint: disable=unused-argument
    driver: webdriver.Chrome,
    create_user_idp: Callable[[str, str, bool, str, str], None],
    idp_login: Callable[[str, str], None],
    get_token: Callable[[str, str], str],
    get_session_context: Callable[[str], str],
) -> None:
    # Create a user in the idp.
    create_user_idp("viewer@saml.integration.test", "password", True)

    # Get the session context from o11y which will give the SAML login URL.
    session_context = get_session_context("viewer@saml.integration.test")

    assert len(session_context["orgs"]) == 1
    assert len(session_context["orgs"][0]["authNSupport"]["callback"]) == 1

    url = session_context["orgs"][0]["authNSupport"]["callback"][0]["url"]

    driver.get(url)
    idp_login("viewer@saml.integration.test", "password")

    admin_token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)

    # Assert that the user was created in o11y.
    found_user = find_user_with_roles_by_email(o11y, admin_token, "viewer@saml.integration.test")
    assert_user_has_role(found_user, "o11y-viewer")


def test_idp_initiated_saml_authn(
    o11y: O11y,
    idp: TestContainerIDP,  # pylint: disable=unused-argument
    driver: webdriver.Chrome,
    create_user_idp: Callable[[str, str, bool, str, str], None],
    idp_login: Callable[[str, str], None],
    get_token: Callable[[str, str], str],
    get_session_context: Callable[[str], str],
) -> None:
    # Create a user in the idp.
    create_user_idp("viewer.idp.initiated@saml.integration.test", "password", True)

    # Get the session context from o11y which will give the SAML login URL.
    session_context = get_session_context("viewer.idp.initiated@saml.integration.test")

    assert len(session_context["orgs"]) == 1
    assert len(session_context["orgs"][0]["authNSupport"]["callback"]) == 1

    idp_initiated_login_url = idp.container.host_configs["6060"].base() + "/realms/master/protocol/saml/clients/idp-initiated-saml-test"

    driver.get(idp_initiated_login_url)
    idp_login("viewer.idp.initiated@saml.integration.test", "password")

    admin_token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)

    # Assert that the user was created in o11y.
    found_user = find_user_with_roles_by_email(o11y, admin_token, "viewer.idp.initiated@saml.integration.test")
    assert_user_has_role(found_user, "o11y-viewer")


def test_saml_update_domain_with_group_mappings(
    o11y: O11y,
    get_token: Callable[[str, str], str],
    get_saml_settings: Callable[[], dict],
) -> None:
    admin_token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)
    domain = get_saml_domain(o11y, admin_token)
    settings = get_saml_settings()

    # update the existing saml domain to have role mappings also
    response = requests.put(
        o11y.self.host_configs["8080"].get(f"/api/v1/domains/{domain['id']}"),
        json={
            "config": {
                "ssoEnabled": True,
                "ssoType": "saml",
                "samlConfig": {
                    "samlEntity": settings["entityID"],
                    "samlIdp": settings["singleSignOnServiceLocation"],
                    "samlCert": settings["certificate"],
                    "attributeMapping": {
                        "name": "givenName",
                        "groups": "groups",
                        "role": "o11y_role",
                    },
                },
                "roleMapping": {
                    "defaultRole": "VIEWER",
                    "groupMappings": {
                        "o11y-admins": "ADMIN",
                        "o11y-editors": "EDITOR",
                        "o11y-viewers": "VIEWER",
                    },
                    "useRoleAttribute": False,
                },
            },
        },
        headers={"Authorization": f"Bearer {admin_token}"},
        timeout=2,
    )

    assert response.status_code == HTTPStatus.NO_CONTENT


def test_saml_role_mapping_single_group_admin(
    o11y: O11y,
    idp: TestContainerIDP,  # pylint: disable=unused-argument
    driver: webdriver.Chrome,
    create_user_idp_with_groups: Callable[[str, str, bool, list[str]], None],
    idp_login: Callable[[str, str], None],
    get_token: Callable[[str, str], str],
    get_session_context: Callable[[str], str],
) -> None:
    """
    Test: User in 'o11y-admins' group gets ADMIN role.
    """
    email = "admin-group-user@saml.integration.test"
    create_user_idp_with_groups(email, "password", True, ["o11y-admins"])

    perform_saml_login(o11y, driver, get_session_context, idp_login, email, "password")

    admin_token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)
    found_user = find_user_with_roles_by_email(o11y, admin_token, email)

    assert_user_has_role(found_user, "o11y-admin")


def test_saml_role_mapping_single_group_editor(
    o11y: O11y,
    idp: TestContainerIDP,  # pylint: disable=unused-argument
    driver: webdriver.Chrome,
    create_user_idp_with_groups: Callable[[str, str, bool, list[str]], None],
    idp_login: Callable[[str, str], None],
    get_token: Callable[[str, str], str],
    get_session_context: Callable[[str], str],
) -> None:
    """
    Test: User in 'o11y-editors' group gets EDITOR role.
    """
    email = "editor-group-user@saml.integration.test"
    create_user_idp_with_groups(email, "password", True, ["o11y-editors"])

    perform_saml_login(o11y, driver, get_session_context, idp_login, email, "password")

    admin_token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)
    found_user = find_user_with_roles_by_email(o11y, admin_token, email)

    assert_user_has_role(found_user, "o11y-editor")


def test_saml_role_mapping_multiple_groups_highest_wins(
    o11y: O11y,
    idp: TestContainerIDP,  # pylint: disable=unused-argument
    driver: webdriver.Chrome,
    create_user_idp_with_groups: Callable[[str, str, bool, list[str]], None],
    idp_login: Callable[[str, str], None],
    get_token: Callable[[str, str], str],
    get_session_context: Callable[[str], str],
) -> None:
    """
    Test: User in multiple groups gets highest role.
    User is in both 'o11y-viewers' and 'o11y-editors'.
    Expected: User gets EDITOR (highest of VIEWER and EDITOR).
    """
    email = f"multi-group-user-{uuid.uuid4().hex[:8]}@saml.integration.test"
    create_user_idp_with_groups(email, "password", True, ["o11y-viewers", "o11y-editors"])

    perform_saml_login(o11y, driver, get_session_context, idp_login, email, "password")

    admin_token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)
    found_user = find_user_with_roles_by_email(o11y, admin_token, email)

    assert_user_has_role(found_user, "o11y-editor")


def test_saml_role_mapping_explicit_viewer_group(
    o11y: O11y,
    idp: TestContainerIDP,  # pylint: disable=unused-argument
    driver: webdriver.Chrome,
    create_user_idp_with_groups: Callable[[str, str, bool, list[str]], None],
    idp_login: Callable[[str, str], None],
    get_token: Callable[[str, str], str],
    get_session_context: Callable[[str], str],
) -> None:
    """
    Test: User explicitly mapped to VIEWER via groups should get VIEWER.
    This tests the bug where VIEWER group mappings were incorrectly ignored.
    """
    email = "viewer-group-user@saml.integration.test"
    create_user_idp_with_groups(email, "password", True, ["o11y-viewers"])

    perform_saml_login(o11y, driver, get_session_context, idp_login, email, "password")

    admin_token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)
    found_user = find_user_with_roles_by_email(o11y, admin_token, email)

    assert_user_has_role(found_user, "o11y-viewer")


def test_saml_role_mapping_unmapped_group_uses_default(
    o11y: O11y,
    idp: TestContainerIDP,  # pylint: disable=unused-argument
    driver: webdriver.Chrome,
    create_user_idp_with_groups: Callable[[str, str, bool, list[str]], None],
    idp_login: Callable[[str, str], None],
    get_token: Callable[[str, str], str],
    get_session_context: Callable[[str], str],
) -> None:
    """
    Test: User in unmapped group falls back to default role (VIEWER).
    """
    email = "unmapped-group-user@saml.integration.test"
    create_user_idp_with_groups(email, "password", True, ["some-other-group"])

    perform_saml_login(o11y, driver, get_session_context, idp_login, email, "password")

    admin_token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)
    found_user = find_user_with_roles_by_email(o11y, admin_token, email)

    assert_user_has_role(found_user, "o11y-viewer")


def test_saml_update_domain_with_use_role_claim(
    o11y: O11y,
    get_token: Callable[[str, str], str],
    get_saml_settings: Callable[[], dict],
) -> None:
    """
    Updates SAML domain to enable useRoleAttribute (direct role attribute).
    """
    admin_token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)
    domain = get_saml_domain(o11y, admin_token)
    settings = get_saml_settings()

    response = requests.put(
        o11y.self.host_configs["8080"].get(f"/api/v1/domains/{domain['id']}"),
        json={
            "config": {
                "ssoEnabled": True,
                "ssoType": "saml",
                "samlConfig": {
                    "samlEntity": settings["entityID"],
                    "samlIdp": settings["singleSignOnServiceLocation"],
                    "samlCert": settings["certificate"],
                    "attributeMapping": {
                        "name": "displayName",
                        "groups": "groups",
                        "role": "o11y_role",
                    },
                },
                "roleMapping": {
                    "defaultRole": "VIEWER",
                    "groupMappings": {
                        "o11y-admins": "ADMIN",
                        "o11y-editors": "EDITOR",
                    },
                    "useRoleAttribute": True,
                },
            },
        },
        headers={"Authorization": f"Bearer {admin_token}"},
        timeout=2,
    )

    assert response.status_code == HTTPStatus.NO_CONTENT


def test_saml_role_mapping_role_claim_takes_precedence(
    o11y: O11y,
    idp: TestContainerIDP,  # pylint: disable=unused-argument
    driver: webdriver.Chrome,
    create_user_idp_with_role: Callable[[str, str, bool, str, list[str]], None],
    idp_login: Callable[[str, str], None],
    get_token: Callable[[str, str], str],
    get_session_context: Callable[[str], str],
    setup_user_profile: Callable[[], None],
) -> None:
    """
    Test: useRoleAttribute takes precedence over group mappings.
    User is in 'o11y-editors' group but has role attribute 'ADMIN'.
    Expected: User gets ADMIN (from role attribute).
    """

    setup_user_profile()

    email = "role-claim-precedence@saml.integration.test"
    create_user_idp_with_role(email, "password", True, "ADMIN", ["o11y-editors"])

    perform_saml_login(o11y, driver, get_session_context, idp_login, email, "password")

    admin_token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)
    found_user = find_user_with_roles_by_email(o11y, admin_token, email)

    assert_user_has_role(found_user, "o11y-admin")


def test_saml_role_mapping_invalid_role_claim_fallback(
    o11y: O11y,
    idp: TestContainerIDP,  # pylint: disable=unused-argument
    driver: webdriver.Chrome,
    create_user_idp_with_role: Callable[[str, str, bool, str, list[str]], None],
    idp_login: Callable[[str, str], None],
    get_token: Callable[[str, str], str],
    get_session_context: Callable[[str], str],
    setup_user_profile: Callable[[], None],
) -> None:
    """
    Test: Invalid role claim falls back to group mappings.
    User has invalid role 'SUPERADMIN' and is in 'o11y-editors'.
    Expected: User gets EDITOR (from group mapping).
    """
    setup_user_profile()
    email = "invalid-role-user@saml.integration.test"
    create_user_idp_with_role(email, "password", True, "SUPERADMIN", ["o11y-editors"])

    perform_saml_login(o11y, driver, get_session_context, idp_login, email, "password")

    admin_token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)
    found_user = find_user_with_roles_by_email(o11y, admin_token, email)

    assert_user_has_role(found_user, "o11y-editor")


def test_saml_role_mapping_case_insensitive(
    o11y: O11y,
    idp: TestContainerIDP,  # pylint: disable=unused-argument
    driver: webdriver.Chrome,
    create_user_idp_with_role: Callable[[str, str, bool, str, list[str]], None],
    idp_login: Callable[[str, str], None],
    get_token: Callable[[str, str], str],
    get_session_context: Callable[[str], str],
    setup_user_profile: Callable[[], None],
) -> None:
    """
    Test: Role attribute matching is case-insensitive.
    User has role 'admin' (lowercase).
    Expected: User gets ADMIN role.
    """
    setup_user_profile()
    email = "lowercase-role-user@saml.integration.test"
    create_user_idp_with_role(email, "password", True, "admin", [])

    perform_saml_login(o11y, driver, get_session_context, idp_login, email, "password")

    admin_token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)
    found_user = find_user_with_roles_by_email(o11y, admin_token, email)

    assert_user_has_role(found_user, "o11y-admin")


def test_saml_name_mapping(
    o11y: O11y,
    idp: TestContainerIDP,  # pylint: disable=unused-argument
    driver: webdriver.Chrome,
    create_user_idp: Callable[[str, str, bool, str, str], None],
    idp_login: Callable[[str, str], None],
    get_token: Callable[[str, str], str],
    get_session_context: Callable[[str], str],
) -> None:
    """Test that user's display name is mapped from SAML displayName attribute."""
    email = "named-user@saml.integration.test"

    create_user_idp(email, "password", True, "Jane", "Smith")

    perform_saml_login(o11y, driver, get_session_context, idp_login, email, "password")

    admin_token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)
    found_user = find_user_with_roles_by_email(o11y, admin_token, email)

    assert found_user["displayName"] == "Jane"  # We are only mapping the first name here
    assert_user_has_role(found_user, "o11y-viewer")


def test_saml_empty_name_fallback(
    o11y: O11y,
    idp: TestContainerIDP,  # pylint: disable=unused-argument
    driver: webdriver.Chrome,
    create_user_idp: Callable[[str, str, bool, str, str], None],
    idp_login: Callable[[str, str], None],
    get_token: Callable[[str, str], str],
    get_session_context: Callable[[str], str],
) -> None:
    """Test that user without displayName in IDP still gets created."""
    email = "no-name@saml.integration.test"

    create_user_idp(email, "password", True)

    perform_saml_login(o11y, driver, get_session_context, idp_login, email, "password")

    admin_token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)
    found_user = find_user_with_roles_by_email(o11y, admin_token, email)

    assert_user_has_role(found_user, "o11y-viewer")


def test_saml_sso_login_activates_pending_invite_user(
    o11y: O11y,
    idp: TestContainerIDP,  # pylint: disable=unused-argument
    driver: webdriver.Chrome,
    create_user_idp_with_groups: Callable[[str, str, bool, list[str]], None],
    idp_login: Callable[[str, str], None],
    get_token: Callable[[str, str], str],
    get_session_context: Callable[[str], str],
) -> None:
    """
    Verify that an invited user (pending_invite) who logs in via SAML SSO is
    auto-activated with the role from the invite, not the SSO default/group role.

    1. Admin invites user as ADMIN
    2. User exists in IDP with 'o11y-viewers' group (would normally get VIEWER)
    3. SSO login activates the user with VIEWER role (SSO Wins)
    """
    email = "sso-pending-invite@saml.integration.test"
    admin_token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)

    # Invite user as ADMIN
    response = requests.post(
        o11y.self.host_configs["8080"].get("/api/v1/invite"),
        json={"email": email, "role": "ADMIN", "name": "SAML SSO Pending User"},
        headers={"Authorization": f"Bearer {admin_token}"},
        timeout=2,
    )
    assert response.status_code == HTTPStatus.CREATED

    # Create IDP user in viewer group — SSO would normally assign VIEWER
    create_user_idp_with_groups(email, "password", True, ["o11y-viewers"])

    perform_saml_login(o11y, driver, get_session_context, idp_login, email, "password")

    # User should be active with VIEWER role from SSO
    found_user = find_user_with_roles_by_email(o11y, admin_token, email)
    assert found_user["status"] == "active"
    assert_user_has_role(found_user, "o11y-viewer")


def test_saml_sso_deleted_user_gets_new_user_on_login(
    o11y: O11y,
    idp: TestContainerIDP,  # pylint: disable=unused-argument
    driver: webdriver.Chrome,
    create_user_idp: Callable[[str, str, bool, str, str], None],
    idp_login: Callable[[str, str], None],
    get_token: Callable[[str, str], str],
    get_session_context: Callable[[str], str],
) -> None:
    """
    Verify the full deleted-user SAML SSO lifecycle:
    1. Invite + activate a user (EDITOR)
    2. Soft delete the user
    3. SSO login attempt — user should remain deleted (blocked)
    5. SSO login — new user should created
    """
    email = "sso-deleted-lifecycle@saml.integration.test"
    admin_token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)

    # --- Step 1: Invite and activate via password reset ---
    response = requests.post(
        o11y.self.host_configs["8080"].get("/api/v1/invite"),
        json={"email": email, "role": "EDITOR", "name": "SAML SSO Lifecycle User"},
        headers={"Authorization": f"Bearer {admin_token}"},
        timeout=2,
    )
    assert response.status_code == HTTPStatus.CREATED
    user_id = response.json()["data"]["id"]
    reset_token = response.json()["data"]["token"]

    response = requests.post(
        o11y.self.host_configs["8080"].get("/api/v1/resetPassword"),
        json={"password": "password123Z$", "token": reset_token},
        timeout=2,
    )
    assert response.status_code == HTTPStatus.NO_CONTENT

    # --- Step 2: Soft delete via DB using API
    response = requests.delete(
        o11y.self.host_configs["8080"].get(f"/api/v1/user/{user_id}"),
        headers={"Authorization": f"Bearer {admin_token}"},
        timeout=2,
    )
    assert response.status_code == HTTPStatus.NO_CONTENT

    # --- Step 3: SSO login should be blocked for deleted user ---
    create_user_idp(email, "password", True, "SAML", "Lifecycle")

    perform_saml_login(o11y, driver, get_session_context, idp_login, email, "password")

    # Verify user is NOT reactivated — check via DB since API may filter deleted users
    with o11y.sqlstore.conn.connect() as conn:
        result = conn.execute(
            sql.text("SELECT status FROM users WHERE id = :user_id"),
            {"user_id": user_id},
        )
        row = result.fetchone()
        assert row is not None
        assert row[0] == "deleted"

    # Verify a NEW active user was auto-provisioned via SSO
    response = requests.get(
        o11y.self.host_configs["8080"].get("/api/v2/users"),
        headers={"Authorization": f"Bearer {admin_token}"},
        timeout=5,
    )
    assert response.status_code == HTTPStatus.OK
    users = response.json()["data"]
    new_user = next(
        (user for user in users if user["email"] == email and user["id"] != user_id),
        None,
    )
    assert new_user is not None
    assert new_user["status"] == "active"
    # Fetch full user with roles to check the assigned role
    response = requests.get(
        o11y.self.host_configs["8080"].get(f"/api/v2/users/{new_user['id']}"),
        headers={"Authorization": f"Bearer {admin_token}"},
        timeout=5,
    )
    assert response.status_code == HTTPStatus.OK
    found_user = response.json()["data"]
    assert_user_has_role(found_user, "o11y-viewer")  # default role from SSO domain config
