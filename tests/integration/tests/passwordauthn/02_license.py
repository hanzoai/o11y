import http
from collections.abc import Callable

import requests
from wiremock.client import (
    HttpMethods,
    Mapping,
    MappingRequest,
    MappingResponse,
    WireMockMatchers,
)

from fixtures import types


def test_apply_license(
    o11y: types.O11y,
    make_http_mocks: Callable[[types.TestContainerDocker, list[Mapping]], None],
    get_token: Callable[[str, str], str],
) -> None:
    make_http_mocks(
        o11y.zeus,
        [
            Mapping(
                request=MappingRequest(
                    method=HttpMethods.GET,
                    url="/v2/licenses/me",
                    headers={"X-O11y-Cloud-Api-Key": {WireMockMatchers.EQUAL_TO: "secret-key"}},
                ),
                response=MappingResponse(
                    status=200,
                    json_body={
                        "status": "success",
                        "data": {
                            "id": "0196360e-90cd-7a74-8313-1aa815ce2a67",
                            "key": "secret-key",
                            "valid_from": 1732146923,
                            "valid_until": -1,
                            "status": "VALID",
                            "state": "EVALUATING",
                            "plan": {
                                "name": "ENTERPRISE",
                            },
                            "platform": "CLOUD",
                            "features": [],
                            "event_queue": {},
                        },
                    },
                ),
                persistent=False,
            )
        ],
    )

    access_token = get_token("admin@integration.test", "password123Z$")

    response = requests.post(
        url=o11y.self.host_configs["8080"].get("/api/v3/licenses"),
        json={"key": "secret-key"},
        headers={"Authorization": "Bearer " + access_token},
        timeout=5,
    )

    assert response.status_code == http.HTTPStatus.ACCEPTED

    response = requests.post(
        url=o11y.zeus.host_configs["8080"].get("/__admin/requests/count"),
        json={"method": "GET", "url": "/v2/licenses/me"},
        timeout=5,
    )

    assert response.json()["count"] == 1


def test_refresh_license(
    o11y: types.O11y,
    make_http_mocks: Callable[[types.TestContainerDocker, list[Mapping]], None],
    get_token: Callable[[str, str], str],
) -> None:
    make_http_mocks(
        o11y.zeus,
        [
            Mapping(
                request=MappingRequest(
                    method=HttpMethods.GET,
                    url="/v2/licenses/me",
                    headers={"X-O11y-Cloud-Api-Key": {WireMockMatchers.EQUAL_TO: "secret-key"}},
                ),
                response=MappingResponse(
                    status=200,
                    json_body={
                        "status": "success",
                        "data": {
                            "id": "0196360e-90cd-7a74-8313-1aa815ce2a67",
                            "key": "secret-key",
                            "valid_from": 1732146922,
                            "valid_until": -1,
                            "status": "VALID",
                            "state": "EVALUATING",
                            "plan": {
                                "name": "ENTERPRISE",
                            },
                            "platform": "CLOUD",
                            "features": [],
                            "event_queue": {},
                        },
                    },
                ),
                persistent=False,
            )
        ],
    )

    access_token = get_token("admin@integration.test", "password123Z$")

    response = requests.put(
        url=o11y.self.host_configs["8080"].get("/api/v3/licenses"),
        headers={"Authorization": "Bearer " + access_token},
        timeout=5,
    )

    assert response.status_code == http.HTTPStatus.NO_CONTENT

    response = requests.get(
        url=o11y.self.host_configs["8080"].get("/api/v3/licenses/active"),
        headers={"Authorization": "Bearer " + access_token},
        timeout=5,
    )
    assert response.status_code == http.HTTPStatus.OK
    assert response.json()["data"]["valid_from"] == 1732146922

    response = requests.post(
        url=o11y.zeus.host_configs["8080"].get("/__admin/requests/count"),
        json={"method": "GET", "url": "/v2/licenses/me"},
        timeout=5,
    )

    assert response.json()["count"] == 1


def test_license_checkout(
    o11y: types.O11y,
    make_http_mocks: Callable[[types.TestContainerDocker, list[Mapping]], None],
    get_token: Callable[[str, str], str],
) -> None:
    make_http_mocks(
        o11y.zeus,
        [
            Mapping(
                request=MappingRequest(
                    method=HttpMethods.POST,
                    url="/v2/subscriptions/me/sessions/checkout",
                    headers={"X-O11y-Cloud-Api-Key": {WireMockMatchers.EQUAL_TO: "secret-key"}},
                ),
                response=MappingResponse(
                    status=200,
                    json_body={
                        "status": "success",
                        "data": {"url": "https://o11y.checkout.com"},
                    },
                ),
                persistent=False,
            )
        ],
    )

    access_token = get_token("admin@integration.test", "password123Z$")

    response = requests.post(
        url=o11y.self.host_configs["8080"].get("/api/v1/checkout"),
        json={"url": "https://integration-o11y.com"},
        headers={"Authorization": "Bearer " + access_token},
        timeout=5,
    )

    assert response.status_code == http.HTTPStatus.CREATED
    assert response.json()["data"]["redirectURL"] == "https://o11y.checkout.com"

    response = requests.post(
        url=o11y.zeus.host_configs["8080"].get("/__admin/requests/count"),
        json={"method": "POST", "url": "/v2/subscriptions/me/sessions/checkout"},
        timeout=5,
    )

    assert response.json()["count"] == 1


def test_license_portal(
    o11y: types.O11y,
    make_http_mocks: Callable[[types.TestContainerDocker, list[Mapping]], None],
    get_token: Callable[[str, str], str],
) -> None:
    make_http_mocks(
        o11y.zeus,
        [
            Mapping(
                request=MappingRequest(
                    method=HttpMethods.POST,
                    url="/v2/subscriptions/me/sessions/portal",
                    headers={"X-O11y-Cloud-Api-Key": {WireMockMatchers.EQUAL_TO: "secret-key"}},
                ),
                response=MappingResponse(
                    status=200,
                    json_body={
                        "status": "success",
                        "data": {"url": "https://o11y.portal.com"},
                    },
                ),
                persistent=False,
            )
        ],
    )

    access_token = get_token("admin@integration.test", "password123Z$")

    response = requests.post(
        url=o11y.self.host_configs["8080"].get("/api/v1/portal"),
        json={"url": "https://integration-o11y.com"},
        headers={"Authorization": "Bearer " + access_token},
        timeout=5,
    )

    assert response.status_code == http.HTTPStatus.CREATED
    assert response.json()["data"]["redirectURL"] == "https://o11y.portal.com"

    response = requests.post(
        url=o11y.zeus.host_configs["8080"].get("/__admin/requests/count"),
        json={"method": "POST", "url": "/v2/subscriptions/me/sessions/portal"},
        timeout=5,
    )

    assert response.json()["count"] == 1
