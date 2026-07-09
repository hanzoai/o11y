from collections.abc import Callable
from datetime import UTC, datetime, timedelta
from http import HTTPStatus

import pytest

from fixtures import types
from fixtures.audit import AuditLog
from fixtures.auth import USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD
from fixtures.querier import (
    BuilderQuery,
    build_logs_aggregation,
    build_order_by,
    build_scalar_query,
    make_query_request,
)


def test_audit_list_all(
    o11y: types.O11y,
    create_user_admin: None,  # pylint: disable=unused-argument
    get_token: Callable[[str, str], str],
    insert_audit_logs: Callable[[list[AuditLog]], None],
) -> None:
    """List audit events across multiple resource types — verify count, ordering, and fields."""
    now = datetime.now(tz=UTC)
    insert_audit_logs(
        [
            AuditLog(
                timestamp=now - timedelta(seconds=3),
                resources={
                    "service.name": "o11y",
                    "o11y.audit.resource.kind": "alert-rule",
                    "o11y.audit.resource.id": "alert-001",
                },
                attributes={
                    "o11y.audit.principal.id": "user-010",
                    "o11y.audit.principal.email": "ops@acme.com",
                    "o11y.audit.principal.type": "user",
                    "o11y.audit.action": "create",
                    "o11y.audit.outcome": "success",
                },
                body="ops@acme.com (user-010) created alert-rule (alert-001)",
                event_name="alert-rule.created",
                severity_text="INFO",
            ),
            AuditLog(
                timestamp=now - timedelta(seconds=2),
                resources={
                    "service.name": "o11y",
                    "o11y.audit.resource.kind": "saved-view",
                    "o11y.audit.resource.id": "view-001",
                },
                attributes={
                    "o11y.audit.principal.id": "user-010",
                    "o11y.audit.principal.email": "ops@acme.com",
                    "o11y.audit.principal.type": "user",
                    "o11y.audit.action": "update",
                    "o11y.audit.outcome": "success",
                },
                body="ops@acme.com (user-010) updated saved-view (view-001)",
                event_name="saved-view.updated",
                severity_text="INFO",
            ),
            AuditLog(
                timestamp=now - timedelta(seconds=1),
                resources={
                    "service.name": "o11y",
                    "o11y.audit.resource.kind": "user",
                    "o11y.audit.resource.id": "user-020",
                },
                attributes={
                    "o11y.audit.principal.id": "user-010",
                    "o11y.audit.principal.email": "ops@acme.com",
                    "o11y.audit.principal.type": "user",
                    "o11y.audit.action": "update",
                    "o11y.audit.action_category": "access_control",
                    "o11y.audit.outcome": "success",
                },
                body="ops@acme.com (user-010) updated user (user-020)",
                event_name="user.role.changed",
                severity_text="INFO",
            ),
        ]
    )
    token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)

    now = datetime.now(tz=UTC)
    response = make_query_request(
        o11y,
        token,
        start_ms=int((now - timedelta(seconds=30)).timestamp() * 1000),
        end_ms=int(now.timestamp() * 1000),
        queries=[
            BuilderQuery(
                signal="logs",
                source="audit",
                limit=100,
                order=[build_order_by("timestamp"), build_order_by("id")],
            ).to_dict()
        ],
        request_type="raw",
    )

    assert response.status_code == HTTPStatus.OK
    assert response.json()["status"] == "success"

    rows = response.json()["data"]["data"]["results"][0]["rows"]
    assert len(rows) == 3

    # Most recent first
    assert rows[0]["data"]["event_name"] == "user.role.changed"
    assert rows[1]["data"]["event_name"] == "saved-view.updated"
    assert rows[2]["data"]["event_name"] == "alert-rule.created"

    # Verify event_name and body are present
    assert rows[0]["data"]["body"] == "ops@acme.com (user-010) updated user (user-020)"
    assert rows[0]["data"]["severity_text"] == "INFO"


@pytest.mark.parametrize(
    "filter_expression,expected_count,expected_event_names",
    [
        pytest.param(
            "o11y.audit.principal.id = 'user-001'",
            3,
            {"session.login", "dashboard.updated", "dashboard.created"},
            id="filter_by_principal_id",
        ),
        pytest.param(
            "o11y.audit.outcome = 'failure'",
            1,
            {"dashboard.deleted"},
            id="filter_by_outcome_failure",
        ),
        pytest.param(
            "o11y.audit.resource.kind = 'dashboard' AND o11y.audit.resource.id = 'dash-001'",
            3,
            {"dashboard.deleted", "dashboard.updated", "dashboard.created"},
            id="filter_by_resource_kind_and_id",
        ),
        pytest.param(
            "o11y.audit.principal.type = 'service_account'",
            1,
            {"serviceaccount.apikey.created"},
            id="filter_by_principal_type",
        ),
        pytest.param(
            "o11y.audit.resource.kind = 'dashboard' AND o11y.audit.action = 'delete'",
            1,
            {"dashboard.deleted"},
            id="filter_by_resource_kind_and_action",
        ),
    ],
)
def test_audit_filter(
    o11y: types.O11y,
    create_user_admin: None,  # pylint: disable=unused-argument
    get_token: Callable[[str, str], str],
    insert_audit_logs: Callable[[list[AuditLog]], None],
    filter_expression: str,
    expected_count: int,
    expected_event_names: set,
) -> None:
    """Parametrized audit filter tests covering the documented query patterns."""
    now = datetime.now(tz=UTC)
    insert_audit_logs(
        [
            AuditLog(
                timestamp=now - timedelta(seconds=5),
                resources={
                    "service.name": "o11y",
                    "o11y.audit.resource.kind": "dashboard",
                    "o11y.audit.resource.id": "dash-001",
                },
                attributes={
                    "o11y.audit.principal.id": "user-001",
                    "o11y.audit.principal.email": "alice@acme.com",
                    "o11y.audit.principal.type": "user",
                    "o11y.audit.action": "create",
                    "o11y.audit.action_category": "configuration_change",
                    "o11y.audit.outcome": "success",
                },
                body="alice@acme.com created dashboard",
                event_name="dashboard.created",
            ),
            AuditLog(
                timestamp=now - timedelta(seconds=4),
                resources={
                    "service.name": "o11y",
                    "o11y.audit.resource.kind": "dashboard",
                    "o11y.audit.resource.id": "dash-001",
                },
                attributes={
                    "o11y.audit.principal.id": "user-001",
                    "o11y.audit.principal.email": "alice@acme.com",
                    "o11y.audit.principal.type": "user",
                    "o11y.audit.action": "update",
                    "o11y.audit.action_category": "configuration_change",
                    "o11y.audit.outcome": "success",
                },
                body="alice@acme.com updated dashboard",
                event_name="dashboard.updated",
            ),
            AuditLog(
                timestamp=now - timedelta(seconds=3),
                resources={
                    "service.name": "o11y",
                    "o11y.audit.resource.kind": "dashboard",
                    "o11y.audit.resource.id": "dash-001",
                },
                attributes={
                    "o11y.audit.principal.id": "user-002",
                    "o11y.audit.principal.email": "viewer@acme.com",
                    "o11y.audit.principal.type": "user",
                    "o11y.audit.action": "delete",
                    "o11y.audit.action_category": "configuration_change",
                    "o11y.audit.outcome": "failure",
                    "o11y.audit.error.type": "forbidden",
                    "o11y.audit.error.code": "authz_forbidden",
                },
                body="viewer@acme.com failed to delete dashboard",
                event_name="dashboard.deleted",
                severity_text="ERROR",
            ),
            AuditLog(
                timestamp=now - timedelta(seconds=2),
                resources={
                    "service.name": "o11y",
                    "o11y.audit.resource.kind": "serviceaccount",
                    "o11y.audit.resource.id": "sa-001",
                },
                attributes={
                    "o11y.audit.principal.id": "sa-001",
                    "o11y.audit.principal.email": "",
                    "o11y.audit.principal.type": "service_account",
                    "o11y.audit.action": "create",
                    "o11y.audit.action_category": "access_control",
                    "o11y.audit.outcome": "success",
                },
                body="sa-001 created serviceaccount",
                event_name="serviceaccount.apikey.created",
            ),
            AuditLog(
                timestamp=now - timedelta(seconds=1),
                resources={
                    "service.name": "o11y",
                    "o11y.audit.resource.kind": "session",
                    "o11y.audit.resource.id": "*",
                },
                attributes={
                    "o11y.audit.principal.id": "user-001",
                    "o11y.audit.principal.email": "alice@acme.com",
                    "o11y.audit.principal.type": "user",
                    "o11y.audit.action": "login",
                    "o11y.audit.action_category": "access_control",
                    "o11y.audit.outcome": "success",
                },
                body="alice@acme.com login session",
                event_name="session.login",
            ),
        ]
    )
    token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)

    now = datetime.now(tz=UTC)
    response = make_query_request(
        o11y,
        token,
        start_ms=int((now - timedelta(seconds=30)).timestamp() * 1000),
        end_ms=int(now.timestamp() * 1000),
        queries=[
            BuilderQuery(
                signal="logs",
                source="audit",
                limit=100,
                filter_expression=filter_expression,
                order=[build_order_by("timestamp"), build_order_by("id")],
            ).to_dict()
        ],
        request_type="raw",
    )

    assert response.status_code == HTTPStatus.OK

    rows = response.json()["data"]["data"]["results"][0]["rows"]
    assert len(rows) == expected_count

    actual_event_names = {row["data"]["event_name"] for row in rows}
    assert actual_event_names == expected_event_names


def test_audit_scalar_count_failures(
    o11y: types.O11y,
    create_user_admin: None,  # pylint: disable=unused-argument
    get_token: Callable[[str, str], str],
    insert_audit_logs: Callable[[list[AuditLog]], None],
) -> None:
    """Alert query — count multiple failures from different principals."""
    now = datetime.now(tz=UTC)
    insert_audit_logs(
        [
            AuditLog(
                timestamp=now - timedelta(seconds=3),
                resources={
                    "service.name": "o11y",
                    "o11y.audit.resource.kind": "dashboard",
                    "o11y.audit.resource.id": "dash-100",
                },
                attributes={
                    "o11y.audit.principal.id": "user-050",
                    "o11y.audit.principal.type": "user",
                    "o11y.audit.action": "delete",
                    "o11y.audit.outcome": "failure",
                },
                body="user-050 failed to delete dashboard",
                event_name="dashboard.deleted",
                severity_text="ERROR",
            ),
            AuditLog(
                timestamp=now - timedelta(seconds=2),
                resources={
                    "service.name": "o11y",
                    "o11y.audit.resource.kind": "alert-rule",
                    "o11y.audit.resource.id": "alert-200",
                },
                attributes={
                    "o11y.audit.principal.id": "user-060",
                    "o11y.audit.principal.type": "user",
                    "o11y.audit.action": "update",
                    "o11y.audit.outcome": "failure",
                },
                body="user-060 failed to update alert-rule",
                event_name="alert-rule.updated",
                severity_text="ERROR",
            ),
            AuditLog(
                timestamp=now - timedelta(seconds=1),
                resources={
                    "service.name": "o11y",
                    "o11y.audit.resource.kind": "dashboard",
                    "o11y.audit.resource.id": "dash-100",
                },
                attributes={
                    "o11y.audit.principal.id": "user-050",
                    "o11y.audit.principal.type": "user",
                    "o11y.audit.action": "update",
                    "o11y.audit.outcome": "success",
                },
                body="user-050 updated dashboard",
                event_name="dashboard.updated",
            ),
        ]
    )
    token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)

    now = datetime.now(tz=UTC)
    response = make_query_request(
        o11y,
        token,
        start_ms=int((now - timedelta(seconds=30)).timestamp() * 1000),
        end_ms=int(now.timestamp() * 1000),
        queries=[
            build_scalar_query(
                name="A",
                signal="logs",
                source="audit",
                aggregations=[build_logs_aggregation("count()")],
                filter_expression="o11y.audit.outcome = 'failure'",
            )
        ],
        request_type="scalar",
    )

    assert response.status_code == HTTPStatus.OK
    assert response.json()["status"] == "success"

    scalar_data = response.json()["data"]["data"]["results"][0].get("data", [])
    assert len(scalar_data) == 1
    assert scalar_data[0][0] == 2


def test_audit_does_not_leak_into_logs(
    o11y: types.O11y,
    create_user_admin: None,  # pylint: disable=unused-argument
    get_token: Callable[[str, str], str],
    insert_audit_logs: Callable[[list[AuditLog]], None],
) -> None:
    """A single audit event in o11y_audit must not appear in regular log queries."""
    now = datetime.now(tz=UTC)
    insert_audit_logs(
        [
            AuditLog(
                timestamp=now - timedelta(seconds=1),
                resources={
                    "service.name": "o11y",
                    "o11y.audit.resource.kind": "organization",
                    "o11y.audit.resource.id": "org-999",
                },
                attributes={
                    "o11y.audit.principal.id": "user-admin",
                    "o11y.audit.principal.type": "user",
                    "o11y.audit.action": "update",
                    "o11y.audit.outcome": "success",
                },
                body="user-admin updated organization (org-999)",
                event_name="organization.updated",
            ),
        ]
    )
    token = get_token(USER_ADMIN_EMAIL, USER_ADMIN_PASSWORD)

    now = datetime.now(tz=UTC)
    response = make_query_request(
        o11y,
        token,
        start_ms=int((now - timedelta(seconds=30)).timestamp() * 1000),
        end_ms=int(now.timestamp() * 1000),
        queries=[
            BuilderQuery(
                signal="logs",
                limit=100,
                order=[build_order_by("timestamp"), build_order_by("id")],
            ).to_dict()
        ],
        request_type="raw",
    )

    assert response.status_code == HTTPStatus.OK

    rows = response.json()["data"]["data"]["results"][0].get("rows") or []

    audit_bodies = [row["data"]["body"] for row in rows if "o11y.audit" in row["data"].get("attributes_string", {}).get("o11y.audit.action", "")]
    assert len(audit_bodies) == 0
