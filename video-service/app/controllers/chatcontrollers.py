import hmac
import json
import logging
from datetime import datetime
from typing import Dict, List

from fastapi import APIRouter, Depends, Header, HTTPException, Query, WebSocket, WebSocketDisconnect
from pydantic import BaseModel, Field, ValidationError
from starlette.concurrency import run_in_threadpool

from app.config import get_settings
from app.repositories.chat import get_chat_repository
from app.schemas.chat import ChatMessageResponse, ChatStats, ChatUser as User

logger = logging.getLogger(__name__)

chat_router = APIRouter(tags=["Chat API"])


async def require_admin_api_key(
    x_admin_api_key: str | None = Header(default=None, alias="X-Admin-API-Key"),
) -> None:
    configured_key = get_settings().admin_api_key
    if not configured_key:
        raise HTTPException(status_code=503, detail="Admin API key is not configured")
    if x_admin_api_key is None or not hmac.compare_digest(x_admin_api_key, configured_key):
        raise HTTPException(status_code=401, detail="Invalid admin API key")


# Pydantic models for API responses



class ChatMessageRequest(BaseModel):
    type: str = Field(pattern="^chat_message$")
    message: str = Field(min_length=1, max_length=500)


class ChatManager:
    def __init__(self, max_users: int = 50):
        self.active_connections: Dict[str, WebSocket] = {}
        self.users: Dict[str, dict] = {}
        self.max_users = max_users
        
    async def connect(self, websocket: WebSocket, user_id: str, username: str):
        """Connect a new user to the chat"""
        if len(self.active_connections) >= self.max_users:
            await websocket.close(code=1000, reason="Chat room is full")
            return False
            
        if user_id in self.active_connections:
            await websocket.close(code=1000, reason="User already connected")
            return False
            
        await websocket.accept()
        self.active_connections[user_id] = websocket
        self.users[user_id] = {
            "username": username,
            "joined_at": datetime.now().isoformat()
        }
        
        # Notify all users about new user
        await self.broadcast({
            "type": "user_joined",
            "user_id": user_id,
            "username": username,
            "timestamp": datetime.now().isoformat(),
            "total_users": len(self.active_connections)
        })
        
        # Send current users list to new user
        await self.send_to_user(user_id, {
            "type": "users_list",
            "users": [{"user_id": uid, "username": user["username"]} 
                     for uid, user in self.users.items()],
            "total_users": len(self.active_connections)
        })
        
        return True
    
    def disconnect(self, user_id: str):
        """Disconnect a user from the chat"""
        username = None
        if user_id in self.active_connections:
            del self.active_connections[user_id]
        if user_id in self.users:
            username = self.users[user_id]["username"]
            del self.users[user_id]
        return username
    
    async def send_to_user(self, user_id: str, message: dict):
        """Send a message to a specific user"""
        if user_id in self.active_connections:
            try:
                await self.active_connections[user_id].send_text(json.dumps(message))
            except:
                # Connection is broken, remove it
                self.disconnect(user_id)
    
    async def broadcast(self, message: dict, exclude_user: str = None):
        """Broadcast a message to all connected users"""
        disconnected_users = []
        for user_id, websocket in list(self.active_connections.items()):
            if user_id != exclude_user:
                try:
                    await websocket.send_text(json.dumps(message))
                except:
                    disconnected_users.append(user_id)
        
        # Clean up disconnected users
        for user_id in disconnected_users:
            self.disconnect(user_id)

# Initialize chat manager
manager = ChatManager()

@chat_router.get("/chat/info", response_model=dict)
async def chat_root():
    """Root endpoint with API information"""
    return {
        "message": "FastAPI Chat API",
        "version": "1.0.0",
        "endpoints": {
            "websocket": "/ws/{user_id}/{username}",
            "messages": "/api/messages",
            "stats": "/api/stats",
            "health": "/health"
        },
        "max_users": manager.max_users,
        "current_users": len(manager.active_connections)
    }

@chat_router.websocket("/ws/{user_id}/{username}")
async def websocket_endpoint(websocket: WebSocket, user_id: str, username: str):
    """WebSocket endpoint for real-time chat communication"""
    username = username.strip()
    user_id = user_id.strip()

    if not username or len(username) > 20 or not user_id or len(user_id) > 50:
        await websocket.close(code=1000, reason="Invalid username or user_id")
        return
    
    # Try to connect user
    connected = await manager.connect(websocket, user_id, username)
    if not connected:
        return
    
    try:
        while True:
            # Receive message from client
            data = await websocket.receive_text()
            message_data = json.loads(data)
            if not isinstance(message_data, dict):
                await websocket.close(code=1003, reason="Expected a JSON object")
                break

            if message_data.get("type") == "chat_message":
                try:
                    request = ChatMessageRequest.model_validate(message_data)
                except ValidationError:
                    await manager.send_to_user(user_id, {
                        "type": "error",
                        "message": "Chat messages must contain 1 to 500 characters.",
                    })
                    continue

                message_text = request.message.strip()
                if not message_text:
                    await manager.send_to_user(user_id, {
                        "type": "error",
                        "message": "Chat messages cannot be empty.",
                    })
                    continue

                chat_message = await run_in_threadpool(
                    get_chat_repository().create_message,
                    user_id,
                    manager.users[user_id]["username"],
                    message_text,
                )
                await manager.broadcast(chat_message)
                
            elif message_data.get("type") == "ping":
                # Respond to ping for connection health check
                await manager.send_to_user(user_id, {
                    "type": "pong",
                    "timestamp": datetime.now().isoformat()
                })
                
    except WebSocketDisconnect:
        pass
    except json.JSONDecodeError:
        await websocket.close(code=1003, reason="Invalid JSON message")
    except Exception as e:
        print(f"WebSocket error for user {user_id}: {e}")
        await websocket.close(code=1011, reason="Internal server error")
    finally:
        # Handle disconnection
        username = manager.disconnect(user_id)
        if username:
            await manager.broadcast({
                "type": "user_left",
                "user_id": user_id,
                "username": username,
                "timestamp": datetime.now().isoformat(),
                "total_users": len(manager.active_connections)
            })

@chat_router.get("/api/stats", response_model=ChatStats)
async def get_chat_stats():
    """Get current chat statistics and active users"""
    return ChatStats(
        active_users=len(manager.active_connections),
        max_users=manager.max_users,
        users=[
            User(
                user_id=user_id,
                username=user_data["username"],
                joined_at=user_data["joined_at"]
            )
            for user_id, user_data in manager.users.items()
        ]
    )

@chat_router.get("/api/users", response_model=List[User])
async def get_active_users():
    """Get list of currently active users"""
    return [
        User(
            user_id=user_id,
            username=user_data["username"],
            joined_at=user_data["joined_at"]
        )
        for user_id, user_data in manager.users.items()
    ]


@chat_router.get("/api/messages", response_model=List[ChatMessageResponse])
def get_chat_messages(
    limit: int = Query(default=50, ge=1, le=100),
    before_id: int | None = Query(default=None, gt=0),
):
    """Return persisted chat history, oldest to newest within the requested page."""
    return get_chat_repository().get_messages(limit=limit, before_id=before_id)


@chat_router.post("/api/broadcast")
async def broadcast_system_message(
    message: str = Query(min_length=1, max_length=500),
    sender: str = Query(default="System", min_length=1, max_length=50),
    _: None = Depends(require_admin_api_key),
):
    """Broadcast a system message to all connected users (admin endpoint)"""
    message = message.strip()
    sender = sender.strip()
    if not message or not sender:
        raise HTTPException(status_code=400, detail="Message cannot be empty")

    system_message = await run_in_threadpool(
        get_chat_repository().create_message,
        "system",
        sender,
        message,
        "system_message",
    )
    system_message["sender"] = system_message.pop("username")
    system_message["total_users"] = len(manager.active_connections)
    await manager.broadcast(system_message)
    logger.info("Admin broadcast sent successfully", extra={"recipients": len(manager.active_connections)})

    return {
        "status": "success",
        "message": "System message broadcasted",
        "recipients": len(manager.active_connections)
    }

@chat_router.delete("/api/users/{target_user_id}")
async def kick_user(target_user_id: str, _: None = Depends(require_admin_api_key)):
    """Remove a user from the chat (admin endpoint)"""
    if target_user_id not in manager.active_connections:
        raise HTTPException(status_code=404, detail="User not found")
    
    websocket = manager.active_connections[target_user_id]
    username = manager.users.get(target_user_id, {}).get("username", "Unknown")
    
    logger.info(f"User {username} (ID: {target_user_id}) is being kicked.")
    try:
        await websocket.close(code=1000, reason="Removed by administrator")
    except Exception:
        logger.exception("Error closing kicked user's websocket", extra={"user_id": target_user_id})
        raise HTTPException(status_code=500, detail="Could not close user's connection")

    manager.disconnect(target_user_id)

    return {
        "status": "success",
        "message": f"User {username} has been removed",
        "user_id": target_user_id
    }