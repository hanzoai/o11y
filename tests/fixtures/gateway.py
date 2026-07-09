import json

import requests
from wiremock.client import WireMockMatchers

from fixtures import types

TEST_KEY_ID = "test-key-id-001"
TEST_LIMIT_ID = "test-limit-id-001"


def common_gateway_headers():
    """Common headers expected on requests forwarded to the gateway."""
    return {
        "X-O11y-Cloud-Api-Key": {WireMockMatchers.EQUAL_TO: "secret-key"},
        "X-Consumer-Username": {WireMockMatchers.EQUAL_TO: "lid:00000000-0000-0000-0000-000000000000"},
        "X-Consumer-Groups": {WireMockMatchers.EQUAL_TO: "ns:default"},
    }


def get_gateway_requests(o11y: types.O11y, method: str, url: str) -> list:
    """Return captured requests from the WireMock gateway journal.

    Returns an empty list when no requests match or the admin API is unreachable.
    """
    response = requests.post(
        o11y.gateway.host_configs["8080"].get("/__admin/requests/find"),
        json={"method": method, "url": url},
        timeout=5,
    )
    return response.json().get("requests", [])


def get_latest_gateway_request_body(o11y: types.O11y, method: str, url: str) -> dict | None:
    """Return the parsed JSON body of the most recent matching gateway request.

    WireMock returns requests in reverse chronological order, so ``matched[0]``
    is always the latest.  Returns ``None`` when no matching request is found.
    """
    matched = get_gateway_requests(o11y, method, url)
    if not matched:
        return None
    return json.loads(matched[0]["body"])
