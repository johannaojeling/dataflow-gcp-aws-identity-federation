FROM golang:1.18-buster as builder

WORKDIR /pipeline

COPY go.* ./
RUN go mod download

COPY main.go ./

ENV GOOS linux
ENV GOARCH amd64

RUN go build -v -o gen-aws-creds


FROM apache/beam_go_sdk:2.42.0

COPY --chmod=0755 entrypoint.sh /pipeline/entrypoint.sh
COPY --from=builder /pipeline/gen-aws-creds /pipeline/gen-aws-creds

ARG AWS_ROLE_ARN
ENV AWS_ROLE_ARN $AWS_ROLE_ARN

ARG AWS_REGION
ENV AWS_REGION $AWS_REGION

ENTRYPOINT ["/pipeline/entrypoint.sh"]
