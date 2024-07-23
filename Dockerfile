FROM golang:1.22.4 as build-h265_transcoder

WORKDIR /app

COPY cmd ./cmd
COPY mediamtx ./mediamtx
COPY go.* *.go ./

RUN CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build -o build/h265_transcoder_linux_amd64 cmd/main.go
RUN CGO_ENABLED=0 GOARCH=arm64 GOOS=linux go build -o build/h265_transcoder_linux_arm64 cmd/main.go

RUN CGO_ENABLED=0 GOARCH=amd64 GOOS=windows go build -o build/h265_transcoder_windows_amd64 cmd/main.go
RUN CGO_ENABLED=0 GOARCH=arm64 GOOS=windows go build -o build/h265_transcoder_windows_arm64 cmd/main.go

RUN CGO_ENABLED=0 GOARCH=amd64 GOOS=darwin go build -o build/h265_transcoder_darwin_amd64 cmd/main.go
RUN CGO_ENABLED=0 GOARCH=arm64 GOOS=darwin go build -o build/h265_transcoder_darwin_arm64 cmd/main.go

RUN chmod u+x build/*

FROM scratch as export-h265_transcoder
COPY --from=build-h265_transcoder /app/build/* /

FROM export-h265_transcoder as start-h265_transcoder

ENTRYPOINT ["/h265_transcoder"]