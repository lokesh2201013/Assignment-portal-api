CREATE TABLE IF NOT EXISTS videos (
    id BIGSERIAL PRIMARY KEY,
    url TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'queued',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS videos_status_idx ON videos (status);

CREATE TABLE IF NOT EXISTS courses (
    id BIGSERIAL PRIMARY KEY, title TEXT NOT NULL, description TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('draft', 'active', 'archived')), created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
ALTER TABLE courses ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'active';
ALTER TABLE courses ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
CREATE TABLE IF NOT EXISTS lessons (
    id BIGSERIAL PRIMARY KEY, course_id BIGINT NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
    video_id BIGINT NOT NULL UNIQUE REFERENCES videos(id) ON DELETE CASCADE,
    title TEXT NOT NULL, position INTEGER NOT NULL DEFAULT 0, transcript TEXT NOT NULL DEFAULT '', created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS lessons_course_position_idx ON lessons(course_id, position);
CREATE TABLE IF NOT EXISTS course_enrollments (
    course_id BIGINT NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
    learner_id TEXT NOT NULL,
    enrolled_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (course_id, learner_id)
);
CREATE INDEX IF NOT EXISTS course_enrollments_learner_idx ON course_enrollments (learner_id, course_id);
CREATE TABLE IF NOT EXISTS video_progress (
    video_id BIGINT NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    learner_id TEXT NOT NULL, resume_position_seconds DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (resume_position_seconds >= 0),
    completed_at TIMESTAMPTZ, updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), PRIMARY KEY (video_id, learner_id)
);

CREATE TABLE IF NOT EXISTS video_processing_incidents (
    id BIGSERIAL PRIMARY KEY,
    video_id BIGINT NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('ingestion', 'processing', 'delivery')),
    detail TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'resolved')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS chat_messages (
    id BIGSERIAL PRIMARY KEY,
    user_id TEXT NOT NULL,
    username TEXT NOT NULL,
    message TEXT NOT NULL CHECK (char_length(message) BETWEEN 1 AND 500),
    message_type TEXT NOT NULL DEFAULT 'chat_message'
        CHECK (message_type IN ('chat_message', 'system_message')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS chat_messages_created_at_idx
    ON chat_messages (created_at DESC, id DESC);
