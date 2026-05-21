.PHONY: help dev prod build clean logs stop restart

COMPOSE_FILE=docker-compose.yml

help:
	@grep -E '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

dev: ## Start dev environment
	docker compose -f $(COMPOSE_FILE) up -d

dev-logs: ## View dev logs
	docker compose -f $(COMPOSE_FILE) logs -f

prod: ## Start prod environment (requires .env)
	@test -f .env || (echo "ERROR: .env not found! Run: cp .env.example .env"; exit 1)
	docker compose -f $(COMPOSE_FILE) --env-file .env up -d

prod-logs: ## View prod logs
	docker compose -f $(COMPOSE_FILE) logs -f

prod-logs-app: ## View app logs
	docker compose -f $(COMPOSE_FILE) logs -f app

logs: ## Alias for dev-logs
	$(MAKE) dev-logs

stop: ## Stop all containers
	docker compose -f $(COMPOSE_FILE) down

restart: ## Restart containers
	docker compose -f $(COMPOSE_FILE) down
	docker compose -f $(COMPOSE_FILE) up -d

clean: ## Stop and remove volumes
	docker compose -f $(COMPOSE_FILE) down -v

build: ## Rebuild Docker images
	docker compose -f $(COMPOSE_FILE) build --no-cache

db-shell: ## Open MySQL shell
	docker compose -f $(COMPOSE_FILE) exec mysql mysql -u deadnav_user -p deadnav

db-backup: ## Create DB backup
	@mkdir -p backups
	docker compose -f $(COMPOSE_FILE) exec -T mysql mysqldump -u deadnav_user -pdeadnav_password deadnav > backups/deadnav_$$(date +%Y%m%d_%H%M%S).sql

health: ## Check health
	@curl -sf http://localhost:8080/health && echo "OK" || echo "FAILED"
