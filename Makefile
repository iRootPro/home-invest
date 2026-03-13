.PHONY: build run dev clean css css-watch setup seed deploy deploy-logs

TAILWIND = ./bin/tailwindcss

setup:
	mkdir -p bin
	curl -sL https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-macos-arm64 -o $(TAILWIND)
	chmod +x $(TAILWIND)

css:
	$(TAILWIND) -i static/css/input.css -o static/css/app.css --minify

css-watch:
	$(TAILWIND) -i static/css/input.css -o static/css/app.css --watch

build: css
	go build -o banki ./cmd/banki/

run: build
	./banki

dev:
	go run ./cmd/banki/

seed:
	sqlite3 banki.db < seed.sql

clean:
	rm -f banki banki.db

DEPLOY_HOST = 192.168.1.165
DEPLOY_USER = root
IMAGE_NAME = banki

deploy:
	rsync -az --delete --filter=':- .dockerignore' . $(DEPLOY_USER)@$(DEPLOY_HOST):~/banki/src/
	scp docker-compose.yml $(DEPLOY_USER)@$(DEPLOY_HOST):~/banki/
	ssh $(DEPLOY_USER)@$(DEPLOY_HOST) "cd ~/banki/src && docker build -t $(IMAGE_NAME):latest . && cd ~/banki && docker compose up -d"

deploy-logs:
	ssh $(DEPLOY_USER)@$(DEPLOY_HOST) "cd ~/banki && docker compose logs -f --tail=100"
