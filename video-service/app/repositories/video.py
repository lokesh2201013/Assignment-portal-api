import logging

from psycopg2 import errorcodes
from psycopg2 import errors as pg_errors

from app.database.postgres import Database
from app.exceptions import ConflictError, DatabaseError, NotFoundError
from app.models import Video
from app.repositories import VideoRepository

logger = logging.getLogger(__name__)


class PostgresVideoRepository(VideoRepository):
    def __init__(self, db: Database):
        self._db = db

    def get_by_id(self, video_id: int) -> Video | None:
        query = """
            SELECT id, url, title, description, status
            FROM videos
            WHERE id = %(id)s
        """
        try:
            with self._db.cursor() as (conn, cur):
                cur.execute(query, {"id": video_id})
                row = cur.fetchone()
        except DatabaseError:
            raise
        except Exception:
            logger.exception("Unexpected error fetching video", extra={"video_id": video_id})
            raise DatabaseError() from None

        if row is None:
            return None
        return Video(
            id=row["id"],
            url=row["url"],
            title=row["title"],
            description=row["description"],
            status=row["status"],
        )

    def update_status(self, video_id: int, status: str) -> None:
        query = """
            UPDATE videos
            SET status = %(status)s
            WHERE id = %(id)s
        """
        try:
            with self._db.cursor() as (conn, cur):
                cur.execute(query, {"id": video_id, "status": status})
                if cur.rowcount == 0:
                    conn.rollback()
                    raise NotFoundError("Video not found")
                conn.commit()
        except NotFoundError:
            raise
        except pg_errors.UniqueViolation:
            logger.warning("Unique constraint violated updating video", extra={"video_id": video_id})
            raise ConflictError("Video already exists") from None
        except DatabaseError:
            raise
        except Exception:
            logger.exception("Unexpected error updating video status", extra={"video_id": video_id})
            raise DatabaseError() from None


_repo_instance: VideoRepository | None = None


def get_video_repository() -> VideoRepository:
    global _repo_instance
    if _repo_instance is None:
        from app.database.postgres import get_database
        _repo_instance = PostgresVideoRepository(get_database())
    return _repo_instance

