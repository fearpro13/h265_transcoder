.PHONY: build build_rev

build:
	mkdir -p build && \
	docker build --tag h265_transcoder . && \
	docker build --tag h265_transcoder --output=build --target=export-h265_transcoder .

build_rev: build
	zip -j ./build/h265_transcoder_v$(ver)_r$(rev).zip README.md LICENSE api_description.txt build/*