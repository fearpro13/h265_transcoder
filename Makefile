.PHONY: build

build:
	docker build --tag h265_transcoder . && docker build --tag h265_transcoder --output=build --target=export-h265_transcoder  .