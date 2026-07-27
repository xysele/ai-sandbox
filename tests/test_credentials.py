from __future__ import annotations

import importlib.util
import json
import os
import stat
import tempfile
import unittest
from pathlib import Path
from unittest import mock

import deploy

REPO_ROOT = Path(__file__).resolve().parents[1]
HELPER_PATH = (
    REPO_ROOT
    / ".agents"
    / "skills"
    / "ai-sandbox-gateway"
    / "scripts"
    / "gateway_call.py"
)


def load_gateway_helper():
    spec = importlib.util.spec_from_file_location("gateway_call", HELPER_PATH)
    if spec is None or spec.loader is None:
        raise RuntimeError("cannot load gateway helper")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


gateway_call = load_gateway_helper()


class FakeResponse:
    status = 200

    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc_value, traceback):
        return False

    def read(self):
        return b'{"success":true}'


class GatewayHelperCredentialTests(unittest.TestCase):
    def test_loads_structured_and_legacy_credentials(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "credentials.json"
            path.write_text(
                json.dumps(
                    {
                        "gateway_token": " gateway-secret ",
                        "studio_token": " studio-secret ",
                    }
                ),
                encoding="utf-8",
            )
            self.assertEqual(
                gateway_call._load_credentials(str(path)),
                ("gateway-secret", "studio-secret"),
            )

            path.write_text("legacy-gateway-secret\n", encoding="utf-8")
            self.assertEqual(
                gateway_call._load_credentials(str(path)),
                ("legacy-gateway-secret", None),
            )

    def test_merges_studio_token_with_base_and_endpoint_queries(self):
        client = gateway_call.GatewayClient(
            "https://example.test/proxy/7860?existing=base&studio_token=stale",
            "gateway-secret",
            30,
            "studio secret",
        )

        url = client._endpoint_url("/task/status?task_id=task_1")
        parsed = gateway_call.parse.urlsplit(url)
        query = gateway_call.parse.parse_qs(parsed.query)

        self.assertEqual(parsed.path, "/proxy/7860/task/status")
        self.assertEqual(query["existing"], ["base"])
        self.assertEqual(query["task_id"], ["task_1"])
        self.assertEqual(query["studio_token"], ["studio secret"])

    def test_sends_gateway_token_only_in_header(self):
        client = gateway_call.GatewayClient(
            "https://example.test/runtime",
            "gateway-secret",
            30,
            "studio-secret",
        )

        with mock.patch.object(
            gateway_call.request,
            "urlopen",
            return_value=FakeResponse(),
        ) as urlopen:
            result = client.call("/system/info", method="GET")

        sent_request = urlopen.call_args.args[0]
        self.assertTrue(result["success"])
        self.assertEqual(
            sent_request.get_header("X-gateway-token"),
            "gateway-secret",
        )
        self.assertNotIn("gateway-secret", sent_request.full_url)
        self.assertIn("studio_token=studio-secret", sent_request.full_url)


class DeploymentCredentialTests(unittest.TestCase):
    def test_reads_legacy_and_structured_gateway_tokens(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "cs_token.txt"
            with (
                mock.patch.object(deploy, "CREDENTIALS_FILE", path),
                mock.patch.dict(
                    os.environ,
                    {},
                    clear=True,
                ),
            ):
                path.write_text("legacy-secret\n", encoding="utf-8")
                self.assertEqual(
                    deploy._read_gateway_token(),
                    ("legacy-secret", str(path)),
                )

                path.write_text(
                    json.dumps(
                        {
                            "gateway_token": "structured-secret",
                            "studio_token": "studio-secret",
                        }
                    ),
                    encoding="utf-8",
                )
                self.assertEqual(
                    deploy._read_gateway_token(),
                    ("structured-secret", str(path)),
                )

    def test_saves_both_tokens_with_private_permissions(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "cs_token.txt"
            with mock.patch.object(deploy, "CREDENTIALS_FILE", path):
                deploy._save_credentials("gateway-secret", "studio-secret")

            self.assertEqual(
                json.loads(path.read_text(encoding="utf-8")),
                {
                    "gateway_token": "gateway-secret",
                    "studio_token": "studio-secret",
                },
            )
            self.assertEqual(stat.S_IMODE(path.stat().st_mode), 0o600)
            self.assertEqual(list(path.parent.glob(".cs_token.txt.*.tmp")), [])


if __name__ == "__main__":
    unittest.main()
