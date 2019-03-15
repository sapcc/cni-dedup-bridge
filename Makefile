VERSION:= 0.1.0

build:
	GOOS=linux go build -v -o release/dedup-bridge .

.PHONY: release
release: build
ifndef GITHUB_TOKEN
	$(error GITHUB_TOKEN is undefined)
endif
	git push
	github-release sapcc/cni-dedup-bridge v$(VERSION) master "v$(VERSION)" "release/*"
