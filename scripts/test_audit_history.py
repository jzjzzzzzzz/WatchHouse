import importlib.util
from pathlib import Path
import subprocess
import unittest

spec = importlib.util.spec_from_file_location("audit_history", Path(__file__).with_name("audit-history.py"))
audit_history = importlib.util.module_from_spec(spec)
spec.loader.exec_module(audit_history)


class HistoryAuditTests(unittest.TestCase):
    def test_current_history_has_disclosed_nonempty_commits(self):
        result = audit_history.audit(require_complete=False)
        count = int(subprocess.run(["git", "rev-list", "--count", "HEAD"], check=True,
                                   capture_output=True, text=True).stdout)
        self.assertEqual(result["commits"], count)
        self.assertEqual(result["required_days"], 163)
        self.assertGreaterEqual(result["unique_scheduled_days"], 155)
        self.assertTrue(result["first_scheduled"].startswith("2026-04-08T"))
        self.assertLess(result["actual_execution_first"], result["actual_execution_last"])

    def test_required_coverage_is_ceiling_of_ninety_percent(self):
        span = (audit_history.END - audit_history.START).days + 1
        self.assertEqual(span, 181)
        self.assertEqual(audit_history.math.ceil(span * 0.90), 163)


if __name__ == "__main__":
    unittest.main()
