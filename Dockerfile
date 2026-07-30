ARG RM_BASE_IMAGE=debian:trixie-slim
FROM golang:1.26-bookworm AS build
WORKDIR /src
ARG RM_VERSION=dev
ARG RM_COMMIT=unknown
ARG RM_IMAGE_TAG=local
ARG RM_BASE_IMAGE
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags "-s -w \
	-X github.com/jo3qma/ocr-mng/internal/version.Version=${RM_VERSION} \
	-X github.com/jo3qma/ocr-mng/internal/version.Commit=${RM_COMMIT} \
	-X github.com/jo3qma/ocr-mng/internal/version.ImageTag=${RM_IMAGE_TAG} \
	-X github.com/jo3qma/ocr-mng/internal/version.BaseImage=${RM_BASE_IMAGE}" \
	-o /out/rm ./cmd/rm

FROM ${RM_BASE_IMAGE}
RUN apt-get update && apt-get install -y ca-certificates git curl && rm -rf /var/lib/apt/lists/ \
    && dpkg --compare-versions "$(git --version | awk '{print $3}')" ge 2.41
RUN curl -fsSL -o /usr/local/bin/ocr https://github.com/alibaba/open-code-review/releases/latest/download/opencodereview-linux-amd64 \
    && chmod +x /usr/local/bin/ocr
COPY --from=build /out/rm /usr/local/bin/rm
ENV RM_DATA_DIR=/data RM_LISTEN_ADDR=:8080 RM_OCR_BINARY=ocr
VOLUME /data
EXPOSE 8080
ENTRYPOINT ["rm"]
