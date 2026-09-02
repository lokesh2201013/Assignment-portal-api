import os
import asyncio
import logging
from aio_pika import connect_robust, IncomingMessage

logger = logging.getLogger(__name__)

async def handle_message(message: IncomingMessage): 
    async with message.process():
        try:
            body_str = message.body.decode()
            video_id = int(body_str)
            logger.info(f"Received task for video ID: {video_id}")
            
            # Defer the synchronous blocking FFMPEG call to a separate thread
            from app.controllers.controllers import ProcessVideo
            result = await asyncio.to_thread(ProcessVideo, video_id)
            
            logger.info(f"Processing complete for video ID: {video_id}, result: {result}")
        except ValueError:
            logger.error(f"Failed to parse video_id from message body: {message.body}")
        except Exception as e:
            logger.exception(f"Error processing video task from message body: {message.body}. Error: {e}")
            
async def start_consumer():
    QUEUE_NAME = "task_queue"
    RABBITMQ_URL = os.getenv("RABBITMQ_URL", "amqp://guest:guest@rabbitmq:5672/")
    
    try:
        connection = await connect_robust(RABBITMQ_URL)
        channel = await connection.channel()
        queue = await channel.declare_queue(QUEUE_NAME, durable=True)

        logger.info(f"🚀 Listening on RabbitMQ queue '{QUEUE_NAME}'...")
        await queue.consume(handle_message, no_ack=False)
        
        # Keep consumer running conceptually (if not managed by main task)
        # Note: If called within uvicorn startup event, the connection stays open
        # until closed during shutdown.
    except Exception as e:
        logger.error(f"Failed to start RabbitMQ consumer: {e}")