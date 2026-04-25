from aio_pika import connect_robust, IncomingMessage
import os

async def handle_message(message: IncomingMessage): 
    async with message.process():
        video_id = int(message.body.decode())
        print(f"Received task for video ID: {video_id}")
        try:
            from app.controllers.controllers import ProcessVideo
            result = ProcessVideo(video_id, db)
            print(f"Processing complete for video ID: {video_id}, result: {result}")
        except Exception as e:
            print(f"Error processing video ID {video_id}: {e}")
            
async def start_consumer():
    QUEUE_NAME="task_queue"
    RABBITMQ_URL = os.getenv("RABBITMQ_URL", "amqp://guest:guest@192.168.0.100:5672/")
    
    connection = await connect_robust(RABBITMQ_URL)
    channel = await connection.channel()
    queue = await channel.declare_queue(QUEUE_NAME, durable=True)

    print(f"🚀 Listening on RabbitMQ queue '{QUEUE_NAME}'...")
    await queue.consume(lambda msg: handle_message(msg), no_ack=False)