import unittest
from contextlib import contextmanager
from datetime import datetime, timezone
from unittest.mock import MagicMock

from app.repositories.chat import PostgresChatRepository


class TestPostgresChatRepository(unittest.TestCase):
    def setUp(self):
        self.mock_db = MagicMock()
        self.repo = PostgresChatRepository(self.mock_db)
        self.mock_conn = MagicMock()
        self.mock_cursor = MagicMock()

        @contextmanager
        def mock_cursor_context():
            yield self.mock_conn, self.mock_cursor

        self.mock_db.cursor = mock_cursor_context

    def test_create_message_persists_and_serializes(self):
        self.mock_cursor.fetchone.return_value = {
            "id": 1,
            "user_id": "user-1",
            "username": "Student",
            "message": "Hello",
            "message_type": "chat_message",
            "created_at": datetime(2026, 10, 18, tzinfo=timezone.utc),
        }

        message = self.repo.create_message("user-1", "Student", "Hello")

        self.assertEqual(message["id"], 1)
        self.assertEqual(message["type"], "chat_message")
        self.assertEqual(message["timestamp"], "2026-10-18T00:00:00+00:00")
        self.mock_conn.commit.assert_called_once()
        self.mock_cursor.execute.assert_called_once()

    def test_get_messages_returns_chronological_page(self):
        self.mock_cursor.fetchall.return_value = [
            {
                "id": 2,
                "user_id": "user-2",
                "username": "Second",
                "message": "Second message",
                "message_type": "chat_message",
                "created_at": datetime(2026, 10, 18, 0, 1, tzinfo=timezone.utc),
            },
            {
                "id": 1,
                "user_id": "user-1",
                "username": "First",
                "message": "First message",
                "message_type": "chat_message",
                "created_at": datetime(2026, 10, 18, tzinfo=timezone.utc),
            },
        ]

        messages = self.repo.get_messages(limit=2, before_id=3)

        self.assertEqual([message["id"] for message in messages], [1, 2])
        self.mock_cursor.execute.assert_called_once_with(
            unittest.mock.ANY,
            {"before_id": 3, "limit": 2},
        )


if __name__ == "__main__":
    unittest.main()
