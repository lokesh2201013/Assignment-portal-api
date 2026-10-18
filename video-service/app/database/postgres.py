from collections.abc import Generator
from contextlib import contextmanager
import logging

import psycopg2
from psycopg2 import pool
from psycopg2.extras import RealDictCursor
from psycopg2.extensions import connection as PgConnection

from app.config import Settings
from app.exceptions import DatabaseError

logger = logging.getLogger(__name__)


class Database:
    def __init__(self, settings: Settings):
        self._settings = settings
        self._pool: pool.ThreadedConnectionPool | None = None

    def connect(self) -> None:
        logger.info(
            "Initializing database pool",
            extra={"host": self._settings.db_host, "db": self._settings.db_name},
        )
        try:
            self._pool = psycopg2.pool.ThreadedConnectionPool(
                self._settings.db_min_conn,
                self._settings.db_max_conn,
                dbname=self._settings.db_name,
                user=self._settings.db_user,
                password=self._settings.db_password,
                host=self._settings.db_host,
                port=self._settings.db_port,
            )
        except psycopg2.Error:
            logger.exception("Failed to create database connection pool")
            raise DatabaseError() from None

    def close(self) -> None:
        if self._pool is not None and not self._pool.closed:
            self._pool.closeall()
            self._pool = None
            logger.info("Database pool closed")

    @contextmanager
    def connection(self) -> Generator[PgConnection, None, None]:
        if self._pool is None:
            self.connect()
        assert self._pool is not None

        conn = None
        try:
            conn = self._pool.getconn()
            yield conn
        except psycopg2.Error:
            if conn is not None:
                conn.rollback()
            logger.exception("Database error")
            raise DatabaseError() from None
        finally:
            if conn is not None:
                self._pool.putconn(conn)

    @contextmanager
    def cursor(self) -> Generator:
        with self.connection() as conn:
            with conn.cursor(cursor_factory=RealDictCursor) as cur:
                yield conn, cur


_db_instance: Database | None = None


def get_database() -> Database:
    global _db_instance
    if _db_instance is None:
        from app.config import get_settings
        _db_instance = Database(get_settings())
    return _db_instance

