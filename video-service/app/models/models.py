from pydantic import BaseModel
from typing import List
class Video(BaseModel):
    id:int
    url:str
    title:str
    description:str
    status:str
    
class User(BaseModel):
    user_id: str
    username: str
    joined_at: str

class ChatMessage(BaseModel):
    type: str
    user_id: str
    username: str
    message: str
    timestamp: str

class ChatStats(BaseModel):
    active_users: int
    max_users: int
    users: List[User]

class SystemMessage(BaseModel):
    type: str
    user_id: str
    username: str
    timestamp: str
    total_users: int