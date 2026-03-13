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

deploy:
	./deploy.sh

deploy-logs:
	@source deploy.conf && ssh -p $${DEPLOY_PORT:-22} $${DEPLOY_USER}@$${DEPLOY_HOST} "cd $${DEPLOY_PATH} && docker compose logs -f --tail=100"
