from collections.abc import Callable
from http import HTTPStatus
from urllib.parse import urlparse

import requests
from selenium import webdriver
from wiremock.resources.mappings import Mapping

from fixtures.auth import (
    USER_ADMIN_EMAIL,
    USER_ADMIN_PASSWORD,
    add_license,
    assert_user_has_role,
    find_user_with_roles_by_email,
)
from fixtures.idp import (
    get_oidc_domain,
    perform_oidc_login,
)
from fixtures.types import Operation, O11y, TestContainerDocker, TestContainerIDP


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


def test_create_auth_domain(
    o11y: O11y,
    idp: TestContainerIDP,  # pylint: disable=unused-argument
    create_oidc_client: Callable[[str, str], None],
    get_oidc_settings: Callable[[], dict],
    create_user_admin: Callable[[], None],  # pylint: disable=unused-argument
    get_token: Callable[[str, str], str],
) -> None:
    """
    This creates an OIDC auth domain in o11y.
    """
    client_id = f"oidc.integration.test.{o11y.self.host_configs['8080'].address}:{o11y.self.host_configs['8080'].port}"
    # Create a saml client in the idp.
    create_oidc_client(client_id, "/api/v1/complete/oidc")

    # Get the saml settings from keycloak.
    settings = get_oidc_settings(client_id)

    # Create a auth domain in o11y.
    admin_token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)

    response = requests.post(
        o11y.self.host_configs["8080"].get("/api/v1/domains"),
        json={
            "name": "oidc.integration.test",
            "config": {
                "ssoEnabled": True,
                "ssoType": "oidc",
                "oidcConfig": {
                    "clientId": settings["client_id"],
                    "clientSecret": settings["client_secret"],
                    # Change the hostname of the issuer to the internal resolvable hostname of the idp
                    "issuer": f"{idp.container.container_configs['6060'].get(urlparse(settings['issuer']).path)}",
                    "issuerAlias": settings["issuer"],
                    "getUserInfo": True,
                },
            },
        },
        headers={"Authorization": f"Bearer {admin_token}"},
        timeout=2,
    )

    assert response.status_code == HTTPStatus.CREATED


def test_oidc_authn(
    o11y: O11y,
    idp: TestContainerIDP,  # pylint: disable=unused-argument
    driver: webdriver.Chrome,
    create_user_idp: Callable[[str, str, bool, str, str], None],
    idp_login: Callable[[str, str], None],
    get_token: Callable[[str, str], str],
    get_session_context: Callable[[str], str],
) -> None:
    """
    This tests the OIDC authn flow.
    It uses a web browser to login to the idp and then asserts that the user was created in o11y.
    """
    # Create a user in the idp.
    create_user_idp("viewer@oidc.integration.test", "password123", True)

    # Get the session context from o11y which will give the OIDC login URL.
    session_context = get_session_context("viewer@oidc.integration.test")

    assert len(session_context["orgs"]) == 1
    assert len(session_context["orgs"][0]["authNSupport"]["callback"]) == 1

    url = session_context["orgs"][0]["authNSupport"]["callback"][0]["url"]

    # change the url to the external resolvable hostname of the idp
    parsed_url = urlparse(url)
    actual_url = f"{idp.container.host_configs['6060'].get(parsed_url.path)}?{parsed_url.query}"

    driver.get(actual_url)
    idp_login("viewer@oidc.integration.test", "password123")

    admin_token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)

    # Assert that the user was created in o11y.
    found_user = find_user_with_roles_by_email(o11y, admin_token, "viewer@oidc.integration.test")
    assert_user_has_role(found_user, "o11y-viewer")


def test_oidc_update_domain_with_group_mappings(
    o11y: O11y,
    idp: TestContainerIDP,
    get_token: Callable[[str, str], str],
    get_oidc_settings: Callable[[str], dict],
) -> None:
    """
    Updates OIDC domain to add role mapping with group mappings and claim mapping.
    """
    admin_token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)
    domain = get_oidc_domain(o11y, admin_token)
    client_id = f"oidc.integration.test.{o11y.self.host_configs['8080'].address}:{o11y.self.host_configs['8080'].port}"
    settings = get_oidc_settings(client_id)

    response = requests.put(
        o11y.self.host_configs["8080"].get(f"/api/v1/domains/{domain['id']}"),
        json={
            "config": {
                "ssoEnabled": True,
                "ssoType": "oidc",
                "oidcConfig": {
                    "clientId": settings["client_id"],
                    "clientSecret": settings["client_secret"],
                    "issuer": f"{idp.container.container_configs['6060'].get(urlparse(settings['issuer']).path)}",
                    "issuerAlias": settings["issuer"],
                    "getUserInfo": True,
                    "claimMapping": {
                        "email": "email",
                        "name": "name",
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


def test_oidc_role_mapping_single_group_admin(
    o11y: O11y,
    idp: TestContainerIDP,
    driver: webdriver.Chrome,
    create_user_idp_with_groups: Callable[[str, str, bool, list[str]], None],
    idp_login: Callable[[str, str], None],
    get_token: Callable[[str, str], str],
    get_session_context: Callable[[str], str],
) -> None:
    """
    Test: OIDC user in 'o11y-admins' group gets ADMIN role.
    """
    email = "admin-group-user@oidc.integration.test"
    create_user_idp_with_groups(email, "password123", True, ["o11y-admins"])

    perform_oidc_login(o11y, idp, driver, get_session_context, idp_login, email, "password123")

    admin_token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)
    found_user = find_user_with_roles_by_email(o11y, admin_token, email)

    assert_user_has_role(found_user, "o11y-admin")


def test_oidc_role_mapping_single_group_editor(
    o11y: O11y,
    idp: TestContainerIDP,
    driver: webdriver.Chrome,
    create_user_idp_with_groups: Callable[[str, str, bool, list[str]], None],
    idp_login: Callable[[str, str], None],
    get_token: Callable[[str, str], str],
    get_session_context: Callable[[str], str],
) -> None:
    """
    Test: OIDC user in 'o11y-editors' group gets EDITOR role.
    """
    email = "editor-group-user@oidc.integration.test"
    create_user_idp_with_groups(email, "password123", True, ["o11y-editors"])

    perform_oidc_login(o11y, idp, driver, get_session_context, idp_login, email, "password123")

    admin_token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)
    found_user = find_user_with_roles_by_email(o11y, admin_token, email)

    assert_user_has_role(found_user, "o11y-editor")


def test_oidc_role_mapping_multiple_groups_highest_wins(
    o11y: O11y,
    idp: TestContainerIDP,
    driver: webdriver.Chrome,
    create_user_idp_with_groups: Callable[[str, str, bool, list[str]], None],
    idp_login: Callable[[str, str], None],
    get_token: Callable[[str, str], str],
    get_session_context: Callable[[str], str],
) -> None:
    """
    Test: OIDC user in multiple groups gets highest role.
    User is in 'o11y-viewers' and 'o11y-admins'.
    Expected: User gets ADMIN (highest of the two).
    """
    email = "multi-group-user@oidc.integration.test"
    create_user_idp_with_groups(email, "password123", True, ["o11y-viewers", "o11y-admins"])

    perform_oidc_login(o11y, idp, driver, get_session_context, idp_login, email, "password123")

    admin_token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)
    found_user = find_user_with_roles_by_email(o11y, admin_token, email)

    assert_user_has_role(found_user, "o11y-admin")


def test_oidc_role_mapping_explicit_viewer_group(
    o11y: O11y,
    idp: TestContainerIDP,
    driver: webdriver.Chrome,
    create_user_idp_with_groups: Callable[[str, str, bool, list[str]], None],
    idp_login: Callable[[str, str], None],
    get_token: Callable[[str, str], str],
    get_session_context: Callable[[str], str],
) -> None:
    """
    Test: OIDC user explicitly mapped to VIEWER via groups gets VIEWER.
    Tests the bug where VIEWER mappings were ignored.
    """
    email = "viewer-group-user@oidc.integration.test"
    create_user_idp_with_groups(email, "password123", True, ["o11y-viewers"])

    perform_oidc_login(o11y, idp, driver, get_session_context, idp_login, email, "password123")

    admin_token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)
    found_user = find_user_with_roles_by_email(o11y, admin_token, email)

    assert_user_has_role(found_user, "o11y-viewer")


def test_oidc_role_mapping_unmapped_group_uses_default(
    o11y: O11y,
    idp: TestContainerIDP,
    driver: webdriver.Chrome,
    create_user_idp_with_groups: Callable[[str, str, bool, list[str]], None],
    idp_login: Callable[[str, str], None],
    get_token: Callable[[str, str], str],
    get_session_context: Callable[[str], str],
) -> None:
    """
    Test: OIDC user in unmapped group falls back to default role.
    """
    email = "unmapped-group-user@oidc.integration.test"
    create_user_idp_with_groups(email, "password123", True, ["some-other-group"])

    perform_oidc_login(o11y, idp, driver, get_session_context, idp_login, email, "password123")

    admin_token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)
    found_user = find_user_with_roles_by_email(o11y, admin_token, email)

    assert_user_has_role(found_user, "o11y-viewer")


def test_oidc_update_domain_with_use_role_claim(
    o11y: O11y,
    idp: TestContainerIDP,
    get_token: Callable[[str, str], str],
    get_oidc_settings: Callable[[str], dict],
) -> None:
    """
    Updates OIDC domain to enable useRoleClaim.
    """
    admin_token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)
    domain = get_oidc_domain(o11y, admin_token)
    client_id = f"oidc.integration.test.{o11y.self.host_configs['8080'].address}:{o11y.self.host_configs['8080'].port}"
    settings = get_oidc_settings(client_id)

    response = requests.put(
        o11y.self.host_configs["8080"].get(f"/api/v1/domains/{domain['id']}"),
        json={
            "config": {
                "ssoEnabled": True,
                "ssoType": "oidc",
                "oidcConfig": {
                    "clientId": settings["client_id"],
                    "clientSecret": settings["client_secret"],
                    "issuer": f"{idp.container.container_configs['6060'].get(urlparse(settings['issuer']).path)}",
                    "issuerAlias": settings["issuer"],
                    "getUserInfo": True,
                    "claimMapping": {
                        "email": "email",
                        "name": "name",
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


def test_oidc_role_mapping_role_claim_takes_precedence(
    o11y: O11y,
    idp: TestContainerIDP,
    driver: webdriver.Chrome,
    create_user_idp_with_role: Callable[[str, str, bool, str, list[str]], None],
    idp_login: Callable[[str, str], None],
    get_token: Callable[[str, str], str],
    get_session_context: Callable[[str], str],
    setup_user_profile: Callable[[], None],
) -> None:
    """
    Test: useRoleAttribute takes precedence over group mappings.
    User is in 'o11y-editors' group but has role claim 'ADMIN'.
    Expected: User gets ADMIN (from role claim).
    """
    setup_user_profile()
    email = "role-claim-precedence@oidc.integration.test"
    create_user_idp_with_role(email, "password123", True, "ADMIN", ["o11y-editors"])

    perform_oidc_login(o11y, idp, driver, get_session_context, idp_login, email, "password123")

    admin_token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)
    found_user = find_user_with_roles_by_email(o11y, admin_token, email)

    assert_user_has_role(found_user, "o11y-admin")


def test_oidc_role_mapping_invalid_role_claim_fallback(
    o11y: O11y,
    idp: TestContainerIDP,
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
    email = "invalid-role-user@oidc.integration.test"
    create_user_idp_with_role(email, "password123", True, "SUPERADMIN", ["o11y-editors"])

    perform_oidc_login(o11y, idp, driver, get_session_context, idp_login, email, "password123")

    admin_token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)
    found_user = find_user_with_roles_by_email(o11y, admin_token, email)

    assert_user_has_role(found_user, "o11y-editor")


def test_oidc_role_mapping_case_insensitive(
    o11y: O11y,
    idp: TestContainerIDP,
    driver: webdriver.Chrome,
    create_user_idp_with_role: Callable[[str, str, bool, str, list[str]], None],
    idp_login: Callable[[str, str], None],
    get_token: Callable[[str, str], str],
    get_session_context: Callable[[str], str],
    setup_user_profile: Callable[[], None],
) -> None:
    """
    Test: Role claim matching is case-insensitive.
    User has role 'editor' (lowercase).
    Expected: User gets EDITOR role.
    """
    setup_user_profile()
    email = "lowercase-role-user@oidc.integration.test"
    create_user_idp_with_role(email, "password123", True, "editor", [])

    perform_oidc_login(o11y, idp, driver, get_session_context, idp_login, email, "password123")

    admin_token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)
    found_user = find_user_with_roles_by_email(o11y, admin_token, email)

    assert_user_has_role(found_user, "o11y-editor")


def test_oidc_name_mapping(
    o11y: O11y,
    idp: TestContainerIDP,
    driver: webdriver.Chrome,
    create_user_idp: Callable[[str, str, bool, str, str], None],
    idp_login: Callable[[str, str], None],
    get_token: Callable[[str, str], str],
    get_session_context: Callable[[str], dict],
) -> None:
    """Test that user's display name is mapped from IDP name claim."""
    email = "named-user@oidc.integration.test"

    # Create user with explicit first/last name
    create_user_idp(email, "password123", True, first_name="John", last_name="Doe")

    perform_oidc_login(o11y, idp, driver, get_session_context, idp_login, email, "password123")

    admin_token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)
    found_user = find_user_with_roles_by_email(o11y, admin_token, email)

    # Keycloak concatenates firstName + lastName into "name" claim
    assert found_user["displayName"] == "John Doe"
    assert_user_has_role(found_user, "o11y-viewer")  # Default role


def test_oidc_empty_name_uses_fallback(
    o11y: O11y,
    idp: TestContainerIDP,
    driver: webdriver.Chrome,
    create_user_idp: Callable[[str, str, bool, str, str], None],
    idp_login: Callable[[str, str], None],
    get_token: Callable[[str, str], str],
    get_session_context: Callable[[str], dict],
) -> None:
    """Test that user without name in IDP still gets created (may have empty displayName)."""
    email = "no-name@oidc.integration.test"

    # Create user without first/last name
    create_user_idp(email, "password123", True)

    perform_oidc_login(o11y, idp, driver, get_session_context, idp_login, email, "password123")

    admin_token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)
    found_user = find_user_with_roles_by_email(o11y, admin_token, email)

    # User should still be created even with empty name
    assert_user_has_role(found_user, "o11y-viewer")
    # Note: displayName may be empty - this is a known limitation


def test_oidc_sso_login_activates_pending_invite_user(
    o11y: O11y,
    idp: TestContainerIDP,
    driver: webdriver.Chrome,
    create_user_idp_with_groups: Callable[[str, str, bool, list[str]], None],
    idp_login: Callable[[str, str], None],
    get_token: Callable[[str, str], str],
    get_session_context: Callable[[str], str],
) -> None:
    """
    Verify that an invited user (pending_invite) who logs in via OIDC SSO is
    auto-activated with the role from the invite, not the SSO default/group role.

    1. Admin invites user as ADMIN
    2. User exists in IDP with 'o11y-viewers' group (would normally get VIEWER)
    3. SSO login activates the user with VIEWER role (SSO wins)
    """
    email = "sso-pending-invite@oidc.integration.test"
    admin_token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)

    # Invite user as ADMIN
    response = requests.post(
        o11y.self.host_configs["8080"].get("/api/v1/invite"),
        json={"email": email, "role": "ADMIN", "name": "OIDC SSO Pending User"},
        headers={"Authorization": f"Bearer {admin_token}"},
        timeout=2,
    )
    assert response.status_code == HTTPStatus.CREATED

    # Create IDP user in viewer group — SSO would normally assign VIEWER
    create_user_idp_with_groups(email, "password123", True, ["o11y-viewers"])

    perform_oidc_login(o11y, idp, driver, get_session_context, idp_login, email, "password123")

    # User should be active with VIEWER role from SSO, not ADMIN from invite
    found_user = find_user_with_roles_by_email(o11y, admin_token, email)
    assert found_user["status"] == "active"
    assert_user_has_role(found_user, "o11y-viewer")
