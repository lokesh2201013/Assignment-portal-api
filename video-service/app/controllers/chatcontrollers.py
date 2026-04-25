from fastapi import FastAPI, WebSocket, WebSocketDisconnect, HTTPException
from pydantic import BaseModel
from typing import Dict, List, Optional
import json
import asyncio
from datetime import datetime
import uuid
from app.models.models import ChatMessage, ChatStats, User, SystemMessage

app = FastAPI(title="Stateless Chat API", description="A real-time chat API supporting up to 50 users")

# Pydantic models for API responses


# In-memory storage (stateless - resets on server restart)
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
        for user_id, websocket in self.active_connections.items():
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

@app.get("/", response_model=dict)
async def root():
    """Root endpoint with API information"""
    return {
        "message": "FastAPI Chat API",
        "version": "1.0.0",
        "endpoints": {
            "websocket": "/ws/{user_id}/{username}",
            "stats": "/api/stats",
            "health": "/health"
        },
        "max_users": manager.max_users,
        "current_users": len(manager.active_connections)
    }

@app.websocket("/ws/{user_id}/{username}")
async def websocket_endpoint(websocket: WebSocket, user_id: str, username: str):
    """WebSocket endpoint for real-time chat communication"""
    # Validate input
    username = username.strip()[:20]  # Limit username length
    user_id = user_id.strip()[:50]    # Limit user_id length
    
    if not username or not user_id:
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
            
            if message_data.get("type") == "chat_message":
                # Validate message
                message_text = message_data.get("message", "").strip()
                if not message_text:
                    continue
                    
                # Create chat message
                chat_message = {
                    "type": "chat_message",
                    "user_id": user_id,
                    "username": manager.users[user_id]["username"],
                    "message": message_text[:500],  # Limit message length
                    "timestamp": datetime.now().isoformat()
                }
                
                # Broadcast to all users
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

@app.get("/api/stats", response_model=ChatStats)
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

@app.get("/api/users", response_model=List[User])
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

@app.post("/api/broadcast")
async def broadcast_system_message(message: str, sender: str = "System"):
    """Broadcast a system message to all connected users (admin endpoint)"""
    if not message.strip():
        raise HTTPException(status_code=400, detail="Message cannot be empty")
    
    system_message = {
        "type": "system_message",
        "sender": sender,
        "message": message.strip()[:500],
        "timestamp": datetime.now().isoformat(),
        "total_users": len(manager.active_connections)
    }
    
    await manager.broadcast(system_message)
    
    return {
        "status": "success",
        "message": "System message broadcasted",
        "recipients": len(manager.active_connections)
    }

@app.delete("/api/users/{user_id}")
async def kick_user(user_id: str):
    """Remove a user from the chat (admin endpoint)"""
    if user_id not in manager.active_connections:
        raise HTTPException(status_code=404, detail="User not found")
    
    websocket = manager.active_connections[user_id]
    username = manager.users.get(user_id, {}).get("username", "Unknown")
    
    # Close the websocket connection
    await websocket.close(code=1000, reason="Removed by administrator")
    
    # Clean up will be handled by the websocket disconnect handler
    
    return {
        "status": "success",
        "message": f"User {username} has been removed",
        "user_id": user_id
    }

@app.get("/health")
async def health_check():
    """Health check endpoint"""
    return {
        "status": "healthy",
        "timestamp": datetime.now().isoformat(),
        "active_connections": len(manager.active_connections),
        "max_users": manager.max_users
    }

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8000)