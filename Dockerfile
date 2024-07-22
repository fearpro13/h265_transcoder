FROM golang:1.22.4 as build-h265_transcoder

WORKDIR /app

COPY cmd ./cmd
COPY mediamtx ./mediamtx
COPY go.* *.go ./

RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o build/h265_transcoder_linux_amd64
RUN GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o build/h265_transcoder_linux_arm64

RUN GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o build/h265_transcoder_windows_amd64
RUN GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o build/h265_transcoder_windows_arm64

RUN GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -o build/h265_transcoder_darwin_amd64
RUN GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -o build/h265_transcoder_darwin_arm64

FROM scratch as export-h265_transcoder
COPY --from=build-h265_transcoder /app/build/* /

FROM export-h265_transcoder as start-h265_transcoder

ENTRYPOINT ["/h265_transcoder"]