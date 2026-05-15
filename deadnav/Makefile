.PHONY: help dev prod build clean logs stop restart

# Default variables
COMPOSE_DEV=docker-compose.yml
COMPOSE_PROD=docker-compose.prod.yml

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# === Development ===
dev: ## Start development environment
	docker compose -f $(COMPOSE_DEV) up -d

dev-logs: ## View development logs
	docker compose -f $(COMPOSE_DEV) logs -f

# === Production ===
prod: ## Start production environment
	@if [ ! -f .env ]; then \
		echo "ERROR: .env file not found!"; \
		echo "Run: cp .env.example .env"; \
		exit 1; \
	fi
	docker compose -f $(COMPOSE_PROD) up -d

prod-logs: ## View production logs
	docker compose -f $(COMPOSE_PROD) logs -f

prod-logs-api: ## View only API logs
	docker compose -f $(COMPOSE_PROD) logs -f api

# === Common ===
logs: ## View logs (alias for dev-logs)
	$(MAKE) dev-logs

stop: ## Stop all containers
	docker compose -f $(COMPOSE_DEV) down
	docker compose -f $(COMPOSE_PROD) down

restart: ## Restart all containers
	$(MAKE) stop
	$(MAKE) prod

clean: ## Stop and remove all containers + volumes
	docker compose -f $(COMPOSE_DEV) down -v
	docker compose -f $(COMPOSE_PROD) down -v

# === Build ===
build: ## Rebuild Docker images
	docker compose -f $(COMPOSE_PROD) build --no-cache

# === Database ===
db-shell: ## Open MySQL shell
	docker exec -it deadnav_db mysql -u deadnav_user -pdeadnav_password deadnav

db-backup: ## Create database backup
	@mkdir -p backups
	docker exec deadnav_db mysqldump -u deadnav_user -pdeadnav_password deadnav > backups/deadnav_$$(date +%Y%m%d_%H%M%S).sql

db-restore: ## Restore database from backup (usage: make db-restore FILE=backups/deadnav_20240101.sql)
	@if [ -z "$(FILE)" ]; then \
		echo "ERROR: Specify backup file: make db-restore FILE=backups/deadnav_20240101.sql"; \
		exit 1; \
	fi
	docker exec -i deadnav_db mysql -u deadnav_user -pdeadnav_password deadnav < $(FILE)

# === Health ===
health: ## Check service health
	@curl -sf http://localhost:8080/health && echo "OK" || echo "FAILED"
