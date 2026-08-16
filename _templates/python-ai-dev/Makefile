VFLAGS=-v

DOCKER_COMPOSE=cd infra && docker compose

ifdef V
  DEBUG_SUFFIX=--terragrunt-log-level debug --terragrunt-debug
else
  DEBUG_SUFFIX=
endif

create_zsh_empties_in_local:
	touch ./local/.zsh_history

up: create_zsh_empties_in_local
	$(DOCKER_COMPOSE) --profile all up -d

down:
	$(DOCKER_COMPOSE) --profile all down

build:
	$(DOCKER_COMPOSE) build jumpbox

restart: down up

rs: build up shell


check_app_name:
ifndef APP
	$(error APP is not set)
endif

build_app: check_app_name
	$(DOCKER_COMPOSE) build $(APP)

logs:
	$(DOCKER_COMPOSE) logs -f jumpbox

shell:
	$(DOCKER_COMPOSE) exec -ti jumpbox zsh

PHONY: up down restart build logs shell


clean:
	rm -rf .venv
	find . -type d -name "__pycache__" -prune -exec rm -rf {} \;
PHONY: up down restart build logs shell clean check_app_name build_app
