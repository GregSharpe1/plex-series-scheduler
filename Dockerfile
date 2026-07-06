FROM golang:1.23-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o /out/scheduler ./cmd/scheduler

FROM scratch

USER 65532:65532

COPY --from=build /out/scheduler /scheduler

ENTRYPOINT ["/scheduler"]
