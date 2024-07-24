# h265_transcoder
Transcodes h265 to h264 with API control

## Build
    make build - build app for all platforms(win, linux, mac)/(amd64, arm64)

    make build_rev ver=1 rev=1 - same as build, but adds readme, licence and api_description files and packs it to zip archive

When build is complete, all binaries could be found in ./build directory

## Run
    h265_decoder --ex <ffmpeg path> [--gpu] [--http_port=8222] [--rtsp_port=9222] [--udp]

    -ex string
    ffmpeg executable path

    -gpu
    Will use gpu hw acceleration(NVIDIA only)

    -http_port uint
    Http listening port (default 8222)

    -rtsp_port uint
    Rtsp listening port (default 9222)

    -udp
    allow udp usage
