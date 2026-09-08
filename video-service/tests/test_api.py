import unittest
from unittest.mock import patch, MagicMock
from starlette.testclient import TestClient

from app.app import app
from app.exceptions import NotFoundError, ValidationError, ExternalServiceError


class TestAPI(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        # Disable worker and DB connection on test startup
        cls.client = TestClient(app, raise_server_exceptions=False)

    def test_health_check(self):
        response = self.client.get("/health")
        self.assertEqual(response.status_code, 200)
        data = response.json()
        self.assertEqual(data["status"], "ok")
        self.assertEqual(data["service"], "video-service")

    def test_request_id_header(self):
        response = self.client.get("/health", headers={"X-Request-ID": "test-req-123"})
        self.assertEqual(response.status_code, 200)
        self.assertEqual(response.headers.get("X-Request-ID"), "test-req-123")

    @patch("app.routes.routes.get_video_repository")
    def test_serve_video_not_found_mapped_to_404(self, mock_get_repo):
        mock_repo = MagicMock()
        mock_repo.get_by_id.return_value = None
        mock_get_repo.return_value = mock_repo

        response = self.client.get("/video/serve/999")
        self.assertEqual(response.status_code, 404)
        data = response.json()
        self.assertEqual(data["error"], "not_found")
        self.assertIn("Video 999 not found", data["message"])

    def test_app_error_handlers_format(self):
        # Dynamically test exception handler mapping on app
        from fastapi import APIRouter
        test_router = APIRouter()

        @test_router.get("/test/validation")
        def raise_val():
            raise ValidationError("Field 'title' is required")

        @test_router.get("/test/external")
        def raise_ext():
            raise ExternalServiceError("GCS storage unreachable")

        @test_router.get("/test/crash")
        def raise_crash():
            raise RuntimeError("Unexpected zero division bug")

        app.include_router(test_router)

        # Test ValidationError
        res = self.client.get("/test/validation")
        self.assertEqual(res.status_code, 400)
        self.assertEqual(res.json(), {"error": "validation", "message": "Field 'title' is required"})

        # Test ExternalServiceError
        res = self.client.get("/test/external")
        self.assertEqual(res.status_code, 502)
        self.assertEqual(res.json(), {"error": "external_service", "message": "GCS storage unreachable"})

        # Test Generic Exception masking
        res = self.client.get("/test/crash")
        self.assertEqual(res.status_code, 500)
        self.assertEqual(res.json(), {"error": "internal", "message": "An unexpected error occurred"})


if __name__ == "__main__":
    unittest.main()
