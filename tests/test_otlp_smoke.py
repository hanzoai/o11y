from pathlib import Path


def test_o11y_core_files_present():
    root = Path(__file__).resolve().parent.parent
    assert (root / "go.mod").exists()
    assert (root / "hanzo.yml").exists()
    assert (root / "health.go").exists()
    assert (root / "telemetry.go").exists()
