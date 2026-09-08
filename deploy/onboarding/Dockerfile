# Stage 1: build ulpfctl — onboarding shells out to `ulpfctl parser run` to
# test a candidate draft parser against the sample (app/ulpfctl.py; the same
# execution path Parser Workbench and manual debugging use — D-011).
FROM golang:1.23-alpine AS ulpfctl-build
WORKDIR /src
RUN apk add --no-cache git
ENV GOTOOLCHAIN=auto
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/ulpfctl ./cmd/ulpfctl

FROM python:3.11-slim
WORKDIR /app
COPY services/onboarding/requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY services/onboarding/app ./app
COPY --from=ulpfctl-build /out/ulpfctl /app/bin/ulpfctl
ENV ULPFCTL_PATH=/app/bin/ulpfctl
EXPOSE 8001
CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8001"]
