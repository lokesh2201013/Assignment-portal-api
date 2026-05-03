import os
import psycopg2
from psycopg2 import pool
import logging
from contextlib import contextmanager

logger = logging.getLogger(__name__)

DB_HOST = os.getenv("DB_HOST", "localhost")
DB_PORT = os.getenv("DB_PORT", "5432")
DB_USER = os.getenv("DB_USER", "your_user")
DB_PASSWORD = os.getenv("DB_PASSWORD", "your_password")
DB_NAME = os.getenv("DB_NAME", "your_db")

db_pool = None

def init_db_pool():
    global db_pool
    logger.info(f"Initializing database connection pool for {DB_NAME} at {DB_HOST}:{DB_PORT}")
    try:
        db_pool = psycopg2.pool.ThreadedConnectionPool(
            1, 20,
            dbname=DB_NAME,
            user=DB_USER,
            password=DB_PASSWORD,
            host=DB_HOST,
            port=DB_PORT
        )
        if db_pool:
            logger.info("Connection pool created successfully")
    except psycopg2.DatabaseError as e:
        logger.error(f"Error creating connection pool: {e}")
        raise

@contextmanager
def get_db_connection():
    if not db_pool:
        init_db_pool()
        
    conn = None
    try:
        conn = db_pool.getconn()
        yield conn
    except psycopg2.Error as e:
        logger.error(f"Database connection error: {e}")
        raise
    finally:
        if conn is not None:
            db_pool.putconn(conn)
