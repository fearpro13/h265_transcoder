# h265_transcoder
Transcodes h265 to h264 with API control

## Build
    make build
When build is complete, all binaries could be found in ./build directory

## Run
    h265_decoder [--rtsp_port=9222] [--http_port=8222] [--gpu] --ex <ffmpeg path>

    -ex string
    ffmpeg executable path

    -gpu
    Will use gpu hw acceleration(NVIDIA only)

    -http_port uint
    Http listening port (default 8222)

    -rtsp_port uint
    Rtsp listening port (default 9222)