FROM python:3.12-slim

ENV PYTHONUNBUFFERED=1 \
    PIP_NO_CACHE_DIR=1

WORKDIR /app
COPY bot-requirements.txt ./
RUN pip install -r bot-requirements.txt

COPY bot.py ./
EXPOSE 8091
CMD ["python", "/app/bot.py"]
