# syntax=docker/dockerfile:1

FROM golang:1.9.2-alpine

WORKDIR /app

COPY . .

WORKDIR /app/handlers_gen

RUN go build codegen.go
RUN ./codegen ../api.go ../api_handlers.go

WORKDIR /app

EXPOSE 8080

CMD [ "go", "test", "-v" ]
