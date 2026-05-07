from pathlib import Path
import unittest


class WorkflowTests(unittest.TestCase):
    def test_postgres_service_and_integration_scope_are_pinned(self):
        workflow = (Path(__file__).parents[1] / ".github/workflows/test.yml").read_text()
        self.assertIn("postgres:18.6-bookworm@sha256:3725f4e2499eef5134592b3b4ab79a543ed7f8e533b05b5b637af926630f6650", workflow)
        self.assertIn("TestPostgresIntegration|EndToEndMutualTLSDeliveryPostgresAndReceiptRecovery", workflow)
        self.assertNotIn("postgres:latest", workflow)


if __name__ == "__main__":
    unittest.main()
