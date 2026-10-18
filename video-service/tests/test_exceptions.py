import unittest
from app.exceptions import (
    AppError,
    ValidationError,
    NotFoundError,
    UnauthorizedError,
    ForbiddenError,
    ConflictError,
    DatabaseError,
    ExternalServiceError,
)


class TestExceptions(unittest.TestCase):
    def test_default_properties(self):
        err = ValidationError()
        self.assertEqual(err.status_code, 400)
        self.assertEqual(err.error_type, "validation")
        self.assertEqual(err.message, "Invalid request")
        self.assertEqual(str(err), "Invalid request")

    def test_custom_message(self):
        err = NotFoundError("Video with ID 999 does not exist")
        self.assertEqual(err.status_code, 404)
        self.assertEqual(err.error_type, "not_found")
        self.assertEqual(err.message, "Video with ID 999 does not exist")
        self.assertEqual(str(err), "Video with ID 999 does not exist")

    def test_all_status_codes(self):
        self.assertEqual(ValidationError().status_code, 400)
        self.assertEqual(UnauthorizedError().status_code, 401)
        self.assertEqual(ForbiddenError().status_code, 403)
        self.assertEqual(NotFoundError().status_code, 404)
        self.assertEqual(ConflictError().status_code, 409)
        self.assertEqual(DatabaseError().status_code, 500)
        self.assertEqual(ExternalServiceError().status_code, 502)
        self.assertEqual(AppError().status_code, 500)


if __name__ == "__main__":
    unittest.main()
