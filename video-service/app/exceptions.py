class AppError(Exception):
    """Base application error mapped to a safe HTTP response."""

    error_type = "internal"
    status_code = 500
    default_message = "An unexpected error occurred"

    def __init__(self, message: str | None = None):
        self.message = message or self.default_message
        super().__init__(self.message)


class ValidationError(AppError):
    error_type = "validation"
    status_code = 400
    default_message = "Invalid request"


class UnauthorizedError(AppError):
    error_type = "unauthorized"
    status_code = 401
    default_message = "Unauthorized"


class ForbiddenError(AppError):
    error_type = "forbidden"
    status_code = 403
    default_message = "Forbidden"


class NotFoundError(AppError):
    error_type = "not_found"
    status_code = 404
    default_message = "Resource not found"


class ConflictError(AppError):
    error_type = "conflict"
    status_code = 409
    default_message = "Resource conflict"


class DatabaseError(AppError):
    error_type = "database"
    status_code = 500
    default_message = "A database error occurred"


class ExternalServiceError(AppError):
    error_type = "external_service"
    status_code = 502
    default_message = "An external service error occurred"
