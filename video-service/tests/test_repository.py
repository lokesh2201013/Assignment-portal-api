import unittest
from unittest.mock import MagicMock
from contextlib import contextmanager

from app.models import Video
from app.repositories.video import PostgresVideoRepository
from app.exceptions import NotFoundError, DatabaseError


class TestPostgresVideoRepository(unittest.TestCase):
    def setUp(self):
        self.mock_db = MagicMock()
        self.repo = PostgresVideoRepository(self.mock_db)

    def test_get_by_id_found(self):
        mock_cur = MagicMock()
        mock_cur.fetchone.return_value = {
            "id": 1,
            "url": "gs://bucket/vid.mp4",
            "title": "Intro to Go",
            "description": "Lecture 1",
            "status": "completed",
        }
        mock_conn = MagicMock()

        @contextmanager
        def mock_cursor():
            yield mock_conn, mock_cur

        self.mock_db.cursor = mock_cursor

        video = self.repo.get_by_id(1)
        self.assertIsNotNone(video)
        self.assertEqual(video.id, 1)
        self.assertEqual(video.title, "Intro to Go")
        self.assertEqual(video.status, "completed")

    def test_get_by_id_not_found(self):
        mock_cur = MagicMock()
        mock_cur.fetchone.return_value = None
        mock_conn = MagicMock()

        @contextmanager
        def mock_cursor():
            yield mock_conn, mock_cur

        self.mock_db.cursor = mock_cursor

        video = self.repo.get_by_id(999)
        self.assertIsNone(video)

    def test_update_status_success(self):
        mock_cur = MagicMock()
        mock_cur.rowcount = 1
        mock_conn = MagicMock()

        @contextmanager
        def mock_cursor():
            yield mock_conn, mock_cur

        self.mock_db.cursor = mock_cursor

        self.repo.update_status(1, "processing")
        mock_conn.commit.assert_called_once()

    def test_update_status_not_found(self):
        mock_cur = MagicMock()
        mock_cur.rowcount = 0  # No rows updated
        mock_conn = MagicMock()

        @contextmanager
        def mock_cursor():
            yield mock_conn, mock_cur

        self.mock_db.cursor = mock_cursor

        with self.assertRaises(NotFoundError):
            self.repo.update_status(999, "processing")
        mock_conn.rollback.assert_called_once()


if __name__ == "__main__":
    unittest.main()
