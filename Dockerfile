FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /stackryze-edge .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /stackryze-edge /stackryze-edge
ENTRYPOINT ["/stackryze-edge"]
