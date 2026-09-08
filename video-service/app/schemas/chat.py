from pydantic import BaseModel, Field


class ChatUser(BaseModel):
    user_id: str
    username: str
    joined_at: str


class ChatStats(BaseModel):
    active_users: int
    max_users: int
    users: list[ChatUser]


class ChatInfoResponse(BaseModel):
    message: str
    version: str
    endpoints: dict[str, str]
    max_users: int
    current_users: int


class BroadcastRequest(BaseModel):
    message: str = Field(min_length=1, max_length=500)
    sender: str = Field(default="System", max_length=50)


class BroadcastResponse(BaseModel):
    status: str
    message: str
    recipients: int


class KickUserResponse(BaseModel):
    status: str
    message: str
    user_id: str
