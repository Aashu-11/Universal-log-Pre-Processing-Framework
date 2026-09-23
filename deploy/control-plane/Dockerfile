# Stage 1: build logkramactl — the control plane shells out to it for parser
# lint/fixture validation and vault read/prove/verify (see app/logkramactl.py;
# docs/DECISIONS.md D-008). One compiled binary, not a second Python
# reimplementation of the same logic.
FROM golang:1.23-alpine AS logkramactl-build
WORKDIR /src
RUN apk add --no-cache git
ENV GOTOOLCHAIN=auto
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/logkramactl ./cmd/logkramactl

FROM python:3.11-slim
WORKDIR /app
COPY services/control-plane/requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY services/control-plane/app ./app
COPY --from=logkramactl-build /out/logkramactl /app/bin/logkramactl
ENV LOGKRAMACTL_PATH=/app/bin/logkramactl
EXPOSE 8000
CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8000"]
