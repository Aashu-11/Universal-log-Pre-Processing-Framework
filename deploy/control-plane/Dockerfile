# Stage 1: build ulpfctl — the control plane shells out to it for parser
# lint/fixture validation and vault read/prove/verify (see app/ulpfctl.py;
# docs/DECISIONS.md D-008). One compiled binary, not a second Python
# reimplementation of the same logic.
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
COPY services/control-plane/requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY services/control-plane/app ./app
COPY --from=ulpfctl-build /out/ulpfctl /app/bin/ulpfctl
ENV ULPFCTL_PATH=/app/bin/ulpfctl
EXPOSE 8000
CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8000"]
