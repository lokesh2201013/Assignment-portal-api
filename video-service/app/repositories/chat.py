import logging
from typing import Any

from app.database.postgres import Database
from app.exceptions import DatabaseError

logger = logging.getLogger(__name__)


class PostgresChatRepository:
    def __init__(self, db: Database):
        self._db = db

    def create_message(
        self,
        user_id: str,
        username: str,
        message: str,
        message_type: str = "chat_message",
    ) -> dict[str, Any]:
        query = """
            INSERT INTO chat_messages (user_id, username, message, message_type)
            VALUES (%(user_id)s, %(username)s, %(message)s, %(message_type)s)
            RETURNING id, user_id, username, message, message_type, created_at
        """
        try:
            with self._db.cursor() as (conn, cur):
                cur.execute(
                    query,
                    {
                        "user_id": user_id,
                        "username": username,
                        "message": message,
                        "message_type": message_type,
                    },
                )
                row = cur.fetchone()
                conn.commit()
        except DatabaseError:
            raise
        except Exception:
            logger.exception("Unexpected error saving chat message")
            raise DatabaseError() from None

        if row is None:
            logger.error("Chat message insert returned no record")
            raise DatabaseError()
        return self._serialize(row)

    def get_messages(self, limit: int = 50, before_id: int | None = None) -> list[dict[str, Any]]:
        query = """
            SELECT id, user_id, username, message, message_type, created_at
            FROM chat_messages
            WHERE (%(before_id)s IS NULL OR id < %(before_id)s)
            ORDER BY id DESC
            LIMIT %(limit)s
        """
        try:
            with self._db.cursor() as (_, cur):
                cur.execute(query, {"before_id": before_id, "limit": limit})
                rows = cur.fetchall()
        except DatabaseError:
            raise
        except Exception:
            logger.exception("Unexpected error fetching chat history")
            raise DatabaseError() from None

        return [self._serialize(row) for row in reversed(rows)]

    @staticmethod
    def _serialize(row: dict[str, Any]) -> dict[str, Any]:
        created_at = row["created_at"]
        if hasattr(created_at, "isoformat"):
            created_at = created_at.isoformat()
        return {
            "id": row["id"],
            "type": row["message_type"],
            "user_id": row["user_id"],
            "username": row["username"],
            "message": row["message"],
            "timestamp": created_at,
        }


_repo_instance: PostgresChatRepository | None = None


def get_chat_repository() -> PostgresChatRepository:
    global _repo_instance
    if _repo_instance is None:
        from app.database.postgres import get_database

        _repo_instance = PostgresChatRepository(get_database())
    return _repo_instance
