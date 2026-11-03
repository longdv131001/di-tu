FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN rm -rf cmd/server/web && cp -R web cmd/server/web
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/ditu ./cmd/server
FROM scratch
COPY --from=build /out/ditu /ditu
EXPOSE 8080
ENV PORT=8080
ENTRYPOINT ["/ditu"]
