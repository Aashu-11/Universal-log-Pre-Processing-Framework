# Stage 1: build logkramactl — onboarding shells out to `logkramactl parser run` to
# test a candidate draft parser against the sample (app/logkramactl.py; the same
# execution path Parser Workbench and manual debugging use — D-011).
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
COPY services/onboarding/requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY services/onboarding/app ./app
COPY --from=logkramactl-build /out/logkramactl /app/bin/logkramactl
ENV LOGKRAMACTL_PATH=/app/bin/logkramactl
EXPOSE 8001
CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8001"]
