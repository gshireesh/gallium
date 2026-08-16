VFLAGS=-v

DOCKER_COMPOSE=cd infra && docker compose 

ifdef V
  DEBUG_SUFFIX=--terragrunt-log-level debug --terragrunt-debug
else
  DEBUG_SUFFIX=
endif

up:
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
	$(DOCKER_COMPOSE) logs -f

shell:
	$(DOCKER_COMPOSE) exec -ti jumpbox zsh

PHONY: up down restart build logs shell


clean:
	find . -type d -name ".terraform" -prune -exec rm -rf {} \;
	find . -type d -name ".terragrunt-cache" -prune -exec rm -rf {} \;
	find . -type f -name "_backend.tf" -prune -exec rm -rf {} \;
	find . -type f -name "_provider.tf" -prune -exec rm -rf {} \;
	find . -type f -name ".terraform.lock.hcl" -prune -exec rm -rf {} \;

switch_terraform:
	cd $(APP) && tfswitch -b ~/.local/bin/terraform -u

init: check_app_name switch_terraform
	cd $(APP) && terragrunt run-all fmt $(DEBUG_SUFFIX)
	cd $(APP) && terragrunt run-all init $(DEBUG_SUFFIX)

plan: check_app_name switch_terraform
	cd $(APP) && terragrunt run-all plan $(DEBUG_SUFFIX)

apply: check_app_name init
	cd $(APP) && terragrunt run-all apply --terragrunt-non-interactive $(DEBUG_SUFFIX)

destroy: check_app_name switch_terraform
	cd $(APP) && terragrunt run-all destroy --terragrunt-non-interactive $(DEBUG_SUFFIX)

PHONY: up down restart build logs shell destroy apply plan init clean switch_terraform check_app_name build_app shell-prod
