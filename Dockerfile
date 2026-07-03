FROM golang:1.24-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/rm ./cmd/rm

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y ca-certificates git curl && rm -rf /var/lib/apt/lists/*
RUN curl -fsSL -o /usr/local/bin/ocr https://github.com/alibaba/open-code-review/releases/latest/download/opencodereview-linux-amd64 \
    && chmod +x /usr/local/bin/ocr
COPY --from=build /out/rm /usr/local/bin/rm
ENV RM_DATA_DIR=/data RM_LISTEN_ADDR=:8080 RM_OCR_BINARY=ocr
VOLUME /data
EXPOSE 8080
ENTRYPOINT ["rm"]
