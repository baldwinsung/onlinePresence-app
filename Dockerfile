# Built with: CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/onlinepresence-server ./cmd/onlinepresence-server
# The binary is compiled on the host first (same pattern as the rest of the
# VPS fleet), so the image is just alpine + one static binary. Replaced the
# python:3.14-slim + Flask image on 2026-10-23.
FROM --platform=linux/amd64 alpine:3.24
COPY bin/onlinepresence-server /usr/local/bin/onlinepresence-server
USER nobody
ENTRYPOINT ["onlinepresence-server"]
CMD ["serve"]
