class ValidationError(Exception):
    """A request contains invalid application data."""


class NotFoundError(Exception):
    """A requested resource does not exist."""


class ConflictError(Exception):
    """A request conflicts with existing application state."""


class DatabaseError(Exception):
    """A database operation failed."""

    def __init__(self, message: str = "A database operation failed"):
        super().__init__(message)


class ExternalServiceError(Exception):
    """An external service operation failed."""
