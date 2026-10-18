import logging

from psycopg2 import errorcodes
from psycopg2 import errors as pg_errors

from app.database.postgres import Database
from app.exceptions import ConflictError, DatabaseError, NotFoundError
from app.models import Video, Lesson
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

    def list_courses(self):
        with self._db.cursor() as (_, cur):
            cur.execute("SELECT id, title, description, status, created_at, updated_at FROM courses ORDER BY id")
            return list(cur.fetchall())

    def list_lessons(self, course_id: int):
        with self._db.cursor() as (_, cur):
            cur.execute("SELECT id, course_id, video_id, title, position, transcript FROM lessons WHERE course_id=%(course_id)s ORDER BY position, id", {"course_id": course_id})
            return [Lesson(**row) for row in cur.fetchall()]

    def get_lesson(self, video_id: int):
        with self._db.cursor() as (_, cur):
            cur.execute("SELECT id, course_id, video_id, title, position, transcript FROM lessons WHERE video_id=%(video_id)s", {"video_id": video_id})
            row = cur.fetchone()
            return Lesson(**row) if row else None

    def save_progress(self, video_id: int, learner_id: str, position: float, completed: bool):
        query = """INSERT INTO video_progress (video_id, learner_id, resume_position_seconds, completed_at, updated_at)
                   VALUES (%(video_id)s,%(learner_id)s,%(position)s,CASE WHEN %(completed)s THEN NOW() ELSE NULL END,NOW())
                   ON CONFLICT (video_id,learner_id) DO UPDATE SET resume_position_seconds=EXCLUDED.resume_position_seconds,
                   completed_at=CASE WHEN EXCLUDED.completed_at IS NOT NULL THEN EXCLUDED.completed_at ELSE video_progress.completed_at END, updated_at=NOW()
                   RETURNING video_id, learner_id, resume_position_seconds, completed_at, updated_at"""
        with self._db.cursor() as (conn, cur):
            cur.execute(query, {"video_id": video_id, "learner_id": learner_id, "position": position, "completed": completed})
            row = cur.fetchone(); conn.commit(); return row

    def get_progress(self, video_id: int, learner_id: str):
        with self._db.cursor() as (_, cur):
            cur.execute("SELECT video_id, learner_id, resume_position_seconds, completed_at, updated_at FROM video_progress WHERE video_id=%(video_id)s AND learner_id=%(learner_id)s", {"video_id": video_id, "learner_id": learner_id})
            return cur.fetchone()

    def create_course(self, title, description, status):
        with self._db.cursor() as (conn, cur):
            cur.execute("INSERT INTO courses (title, description, status) VALUES (%s, %s, %s) RETURNING id, title, description, status, created_at, updated_at", (title, description, status))
            row = cur.fetchone(); conn.commit(); return row

    def update_course(self, course_id, title, description, status):
        with self._db.cursor() as (conn, cur):
            cur.execute("UPDATE courses SET title=COALESCE(%s,title), description=COALESCE(%s,description), status=COALESCE(%s,status), updated_at=NOW() WHERE id=%s RETURNING id, title, description, status, created_at, updated_at", (title, description, status, course_id))
            row = cur.fetchone()
            if not row: conn.rollback(); raise NotFoundError("Course not found")
            conn.commit(); return row

    def delete_course(self, course_id):
        with self._db.cursor() as (conn, cur):
            cur.execute("DELETE FROM courses WHERE id=%s", (course_id,))
            if cur.rowcount == 0: conn.rollback(); raise NotFoundError("Course not found")
            conn.commit()

    def create_lesson(self, course_id, video_id, title, position, transcript):
        with self._db.cursor() as (conn, cur):
            cur.execute("INSERT INTO lessons (course_id, video_id, title, position, transcript) VALUES (%s,%s,%s,%s,%s) RETURNING id, course_id, video_id, title, position, transcript", (course_id, video_id, title, position, transcript))
            row = cur.fetchone(); conn.commit(); return Lesson(**row)

    def update_lesson(self, lesson_id, title, position, transcript):
        with self._db.cursor() as (conn, cur):
            cur.execute("UPDATE lessons SET title=COALESCE(%s,title), position=COALESCE(%s,position), transcript=COALESCE(%s,transcript) WHERE id=%s RETURNING id, course_id, video_id, title, position, transcript", (title, position, transcript, lesson_id))
            row = cur.fetchone()
            if not row: conn.rollback(); raise NotFoundError("Lesson not found")
            conn.commit(); return Lesson(**row)

    def delete_lesson(self, lesson_id):
        with self._db.cursor() as (conn, cur):
            cur.execute("DELETE FROM lessons WHERE id=%s", (lesson_id,))
            if cur.rowcount == 0: conn.rollback(); raise NotFoundError("Lesson not found")
            conn.commit()

    def is_enrolled(self, course_id, learner_id):
        with self._db.cursor() as (_, cur):
            cur.execute("SELECT EXISTS (SELECT 1 FROM course_enrollments WHERE course_id=%s AND learner_id=%s)", (course_id, learner_id))
            return cur.fetchone()["exists"]

    def enroll(self, course_id, learner_id):
        with self._db.cursor() as (conn, cur):
            cur.execute("INSERT INTO course_enrollments (course_id, learner_id) VALUES (%s,%s) ON CONFLICT DO NOTHING", (course_id, learner_id)); conn.commit()

    def unenroll(self, course_id, learner_id):
        with self._db.cursor() as (conn, cur):
            cur.execute("DELETE FROM course_enrollments WHERE course_id=%s AND learner_id=%s", (course_id, learner_id)); conn.commit()

    def list_enrollments(self, course_id):
        with self._db.cursor() as (_, cur):
            cur.execute("SELECT learner_id, enrolled_at FROM course_enrollments WHERE course_id=%s ORDER BY learner_id", (course_id,)); return list(cur.fetchall())

    def create_incident(self, video_id, kind, detail):
        with self._db.cursor() as (conn, cur):
            cur.execute("INSERT INTO video_processing_incidents (video_id, kind, detail) VALUES (%s,%s,%s) RETURNING id, video_id, kind, detail, status, created_at", (video_id, kind, detail)); row = cur.fetchone(); conn.commit(); return row

    def list_incidents(self, status):
        with self._db.cursor() as (_, cur):
            cur.execute("SELECT id, video_id, kind, detail, status, created_at, resolved_at FROM video_processing_incidents WHERE (%s IS NULL OR status=%s) ORDER BY created_at DESC", (status, status)); return list(cur.fetchall())

    def resolve_incident(self, incident_id):
        with self._db.cursor() as (conn, cur):
            cur.execute("UPDATE video_processing_incidents SET status='resolved', resolved_at=NOW() WHERE id=%s", (incident_id,))
            if cur.rowcount == 0: conn.rollback(); raise NotFoundError("Incident not found")
            conn.commit()


_repo_instance: VideoRepository | None = None


def get_video_repository() -> VideoRepository:
    global _repo_instance
    if _repo_instance is None:
        from app.database.postgres import get_database
        _repo_instance = PostgresVideoRepository(get_database())
    return _repo_instance

