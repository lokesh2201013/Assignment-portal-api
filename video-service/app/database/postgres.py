from contextlib import contextmanager
from threading import Lock
from typing import Iterator

from psycopg2.pool import ThreadedConnectionPool
from psycopg2.extras import RealDictCursor

from app.config import get_settings


class Database:
    """Lazily creates a PostgreSQL connection pool on the first DB operation."""

    def __init__(self):
        self._pool: ThreadedConnectionPool | None = None
        self._pool_lock = Lock()

    def _get_pool(self) -> ThreadedConnectionPool:
        if self._pool is None:
            with self._pool_lock:
                if self._pool is None:
                    settings = get_settings()
                    if not settings.db_name or not settings.db_user:
                        raise RuntimeError("DB_NAME and DB_USER must be configured")
                    self._pool = ThreadedConnectionPool(
                        settings.db_min_conn,
                        settings.db_max_conn,
                        host=settings.db_host,
                        port=settings.db_port,
                        user=settings.db_user,
                        password=settings.db_password,
                        dbname=settings.db_name,
                    )
        return self._pool

    @contextmanager
    def cursor(self) -> Iterator[tuple[object, object]]:
        pool = self._get_pool()
        connection = pool.getconn()
        try:
            with connection.cursor(cursor_factory=RealDictCursor) as cursor:
                yield connection, cursor
        finally:
            pool.putconn(connection)


_database: Database | None = None
_database_lock = Lock()


def get_database() -> Database:
    global _database
    if _database is None:
        with _database_lock:
            if _database is None:
                _database = Database()
    return _database
