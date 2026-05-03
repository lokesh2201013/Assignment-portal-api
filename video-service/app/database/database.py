import os
import psycopg2
import logging
from contextlib import contextmanager

logger = logging.getLogger(__name__)

DB_HOST = os.getenv("DB_HOST", "localhost")
DB_PORT = os.getenv("DB_PORT", "5432")
DB_USER = os.getenv("DB_USER", "your_user")
DB_PASSWORD = os.getenv("DB_PASSWORD", "your_password")
DB_NAME = os.getenv("DB_NAME", "your_db")

def init_db_pool():
    """Returns a connection pool, or we can just use simple connect for now."""
    logger.info(f"Initializing database connection logic for {DB_NAME} at {DB_HOST}:{DB_PORT}")

@contextmanager
def get_db_connection():
    """
    Context manager that yields a postgres connection and automatically closes it.
    Use it with 'with get_db_connection() as conn:'
    """
    conn = None
    try:
        conn = psycopg2.connect(
            dbname=DB_NAME,
            user=DB_USER,
            password=DB_PASSWORD,
            host=DB_HOST,
            port=DB_PORT
        )
        yield conn
    except psycopg2.Error as e:
        logger.error(f"Database connection error: {e}")
        raise
    finally:
        if conn is not None:
            conn.close()

# Temporary test on module import (you might want to remove this in production)
try:
    with get_db_connection() as _test_conn:
        with _test_conn.cursor() as _cur:
            _cur.execute("SELECT version();")
            _record = _cur.fetchone()
            logger.info(f"Connected to Postgres. Version: {_record[0]}")
except Exception as e:
    logger.warning(f"Initial DB connection test failed: {e}")
