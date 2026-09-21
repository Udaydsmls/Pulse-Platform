GO_SERVICES := user-service order-service inventory-service payment-service notification-service

.PHONY: help up down logs test seed

help:
	@echo "make up     build the images and start everything"
	@echo "make down   stop everything"
	@echo "make logs   follow the logs"
	@echo "make test   run the unit tests"
	@echo "make seed   seed stock and place two orders through the saga"

up:
	docker compose up -d --build

down:
	docker compose down

logs:
	docker compose logs -f

test:
	@for svc in $(GO_SERVICES); do \
	  echo "go test services/$$svc"; \
	  (cd services/$$svc && go test ./...) || exit 1; \
	done
	@cd services/api-gateway && npm test

seed:
	bash scripts/seed_data.sh
