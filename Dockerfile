FROM golang:alpine as build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . ./

RUN CGO_ENABLED=0 GOOS=linux go build -o /dst ./cmd

FROM alpine

COPY --from=build /dst /

EXPOSE 8080

CMD ["/dst"]
