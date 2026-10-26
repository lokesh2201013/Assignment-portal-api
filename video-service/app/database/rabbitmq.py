import asyncio
import logging

from aio_pika import IncomingMessage, connect_robust

from app.config import get_settings

logger = logging.getLogger(__name__)


async def handle_message(message: IncomingMessage) -> None:
    async with message.process():
        try:
            video_id = int(message.body.decode())
        except (UnicodeDecodeError, ValueError):
            logger.exception("Ignoring invalid video processing message")
            return

        from app.controllers.controllers import ProcessVideo

        result = await asyncio.to_thread(ProcessVideo, video_id)
        logger.info(
            "Video processing completed",
            extra={"video_id": video_id, "result": result},
        )


async def start_consumer() -> None:
    settings = get_settings()
    connection = await connect_robust(settings.rabbitmq_url)
    try:
        channel = await connection.channel()
        queue = await channel.declare_queue(settings.rabbitmq_queue, durable=True)
        logger.info("Listening on RabbitMQ queue %s", settings.rabbitmq_queue)
        await queue.consume(handle_message, no_ack=False)
        await asyncio.Future()
    finally:
        await connection.close()
