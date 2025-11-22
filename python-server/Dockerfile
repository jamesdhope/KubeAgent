# Python server Dockerfile
FROM python:3.10-slim

WORKDIR /app

COPY app.py ./

RUN pip install flask redis

EXPOSE 8080

CMD ["python", "app.py"]
