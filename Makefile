.PHONY: menu build test run stop status logs
menu:
	go run ./cmd/tgproxy menu
build:
	docker compose build
test:
	go test ./...
run:
	docker compose up -d --build
stop:
	docker compose down
status:
	docker compose ps
logs:
	docker compose logs -f --tail=100
